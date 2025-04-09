package sink

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// InfluxDB sink, use influxdb line protocol
// Ref: https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol

// Transform data to line protocol and sink to influxdb, jq script example for PresenceEvent:
//   - input:  {"thingId":"test", "eventType": "connected", "timestamp": 1711529686403}
//   - jq:     .payload | "presence,thingId=" + .thingId + " v=" + (.eventType=="connected"|tostring) + " " + (.timestamp|tostring)
//   - output: presence,thingId=test v=true 1711529686403

const (
	TypeInfluxDB         = "influxdb" // sink type
	MsgChanSize          = 10000      // channel size for messages
	DefaultBatchSize     = 1000       // default batch size
	DefaultBatchTimeout  = 1000       // default batch timeout (ms)
	DefaultMaxRetries    = 3          // default max retries
	DefaultRetryInterval = 2000       // default retry interval (ms)
)

func init() {
	Register(TypeInfluxDB, NewInfluxDB)
}

type InfluxDBConfig struct {
	// BatchSize is the number of messages to collect before sending
	BatchSize int `mapstructure:"batchSize"`
	// BatchTimeout is the maximum time (milliseconds) to wait before sending a batch
	BatchTimeout int `mapstructure:"batchTimeout"`
	// MaxRetries is the maximum number of retries for failed requests
	MaxRetries int `mapstructure:"maxRetries"`
	// RetryInterval is the interval between retries (milliseconds)
	RetryInterval int `mapstructure:"retryInterval"`
}

func NewInfluxDB(ctx context.Context, name string, cfg map[string]any, conn connector.Conn) (Sink, error) {
	var ac InfluxDBConfig
	if err := mapstructure.Decode(cfg, &ac); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	// Set default values if not configured
	if ac.BatchSize <= 0 {
		ac.BatchSize = DefaultBatchSize
	}
	if ac.BatchTimeout <= 0 {
		ac.BatchTimeout = DefaultBatchTimeout
	}
	if ac.MaxRetries <= 0 {
		ac.MaxRetries = DefaultMaxRetries
	}
	if ac.RetryInterval <= 0 {
		ac.RetryInterval = DefaultRetryInterval
	}

	c, ok := conn.(connector.InfluxDB)
	if !ok {
		return nil, fmt.Errorf("wrong connector type for InfluxDB sink")
	}

	a := &InfluxDBImpl{
		ctx:  ctx,
		name: name,
		cfg:  ac,
		conn: c,
		ch:   make(chan *Msg, MsgChanSize),
	}
	go a.publishLoop()
	return a, nil
}

type InfluxDBImpl struct {
	ctx  context.Context
	name string
	cfg  InfluxDBConfig
	conn connector.InfluxDB
	ch   chan *Msg

	started bool
}

func (s *InfluxDBImpl) Start() error {
	s.started = true
	slog.Info("Rule started sink", "type", s.Type(), "name", s.name)
	return s.Status().Error
}

func (s *InfluxDBImpl) Status() model.StatusInfo {
	if !s.started {
		return model.StatusNotStarted()
	}
	return withConnStatus(s.conn.Name(), s.conn.Status())
}

func (s *InfluxDBImpl) Stop() error {
	s.started = false
	slog.Info("Rule stopped sink", "type", s.Type(), "name", s.name)
	return nil
}

func (s *InfluxDBImpl) Name() string {
	return s.name
}

func (*InfluxDBImpl) Type() string {
	return TypeInfluxDB
}

func (s *InfluxDBImpl) Publish(msg Msg) {
	if s.started {
		s.ch <- &msg
	}
}

func (s *InfluxDBImpl) publishLoop() {
	timeout := time.Duration(s.cfg.BatchTimeout) * time.Millisecond
	batch := make([]string, 0, s.cfg.BatchSize)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.sendBatch(batch)
			return
		case msg := <-s.ch:
			batch = append(batch, msg.Payload)
			if len(batch) >= s.cfg.BatchSize {
				ticker.Reset(timeout)
				batch = s.sendBatch(batch)
			}
		case <-ticker.C:
			batch = s.sendBatch(batch)
		}
	}
}

func (s *InfluxDBImpl) sendBatch(batch []string) []string {
	if len(batch) == 0 {
		return batch
	}

	// Join all lines with newlines for batch writing
	payload := strings.Join(batch, "\n")

	// Implement retry logic
	var lastErr error
	for retry := 0; retry <= s.cfg.MaxRetries; retry++ {
		if retry > 0 {
			slog.Info("Rule sink InfluxDB retrying",
				"retry", retry,
				"maxRetries", s.cfg.MaxRetries,
				"batchSize", len(batch))
			// Wait for the retry interval
			select {
			case <-time.After(time.Duration(s.cfg.RetryInterval) * time.Millisecond):
				// Do nothing
			case <-s.ctx.Done():
				return batch
			}
		}

		r, err := s.conn.Client().R().
			SetContext(s.ctx).
			SetBody(payload).
			Post("")

		if err != nil {
			lastErr = err
			slog.Error("Rule sink InfluxDB post batch data failed",
				"error", err,
				"retry", retry,
				"batchSize", len(batch))
			continue
		}

		if r.IsError() {
			lastErr = fmt.Errorf("http status %d: %s", r.StatusCode(), r.Body())
			slog.Error("Rule sink InfluxDB post batch data failed",
				"httpStatus", r.StatusCode(),
				"responseBody", r.Body(),
				"retry", retry,
				"batchSize", len(batch))
			continue
		}

		// Successfully sent
		slog.Debug("Rule sink InfluxDB post batch data SUCCESS",
			"batchSize", len(batch),
			"retries", retry)

		// Clear the batch
		return batch[:0]
	}

	// All retries failed, record the final error
	slog.Error("Rule sink InfluxDB post batch data failed after retries",
		"error", lastErr,
		"maxRetries", s.cfg.MaxRetries,
		"batchSize", len(batch))

	// Even if failed, clear the batch to avoid infinite loop
	return batch[:0]
}
