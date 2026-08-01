package sink

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"ruff.io/tio/rule/connector"
	"ruff.io/tio/rule/model"
)

// MockInfluxDBConn is a mock implementation of InfluxDB connector
type MockInfluxDBConn struct {
	mock.Mock
	connector.InfluxDB
}

func (m *MockInfluxDBConn) Name() string {
	return "mock-influxdb"
}

func (m *MockInfluxDBConn) Type() string {
	return "influxdb"
}

func (m *MockInfluxDBConn) Status() model.StatusInfo {
	return model.StatusInfo{Status: model.Connected}
}

func (m *MockInfluxDBConn) Client() *resty.Client {
	args := m.Called()
	return args.Get(0).(*resty.Client)
}

// MockRestyRequest is a mock implementation of resty.Request
type MockRestyRequest struct {
	mock.Mock
	callCount int
	mu        sync.Mutex
}

func NewMockRestyRequest() *MockRestyRequest {
	return &MockRestyRequest{
		callCount: 0,
	}
}

func (m *MockRestyRequest) incrementCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.callCount
}

func (m *MockRestyRequest) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *MockRestyRequest) SetContext(ctx context.Context) any {
	args := m.Called(ctx)
	return args.Get(0)
}

func (m *MockRestyRequest) SetBody(body any) any {
	args := m.Called(body)
	return args.Get(0)
}

func (m *MockRestyRequest) Post(url string) (*resty.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*resty.Response), args.Error(1)
}

func TestNewInfluxDB(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		wantErr bool
	}{
		{
			name: "default config",
			cfg:  map[string]any{},
		},
		{
			name: "custom batch size",
			cfg: map[string]any{
				"batchSize": 500,
			},
		},
		{
			name: "custom batch timeout",
			cfg: map[string]any{
				"batchTimeout": 2 * time.Second,
			},
		},
		{
			name: "invalid batch timeout",
			cfg: map[string]any{
				"batchTimeout": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn := &MockInfluxDBConn{}
			conn.On("Client").Return(resty.New())
			sink, err := NewInfluxDB(ctx, "test-sink", tt.cfg, conn, nil)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, sink)
			assert.Equal(t, "test-sink", sink.Name())
			assert.Equal(t, TypeInfluxDB, sink.Type())
		})
	}
}

func TestInfluxDBPublish(t *testing.T) {
	tests := []struct {
		name         string
		batchSize    int
		batchTimeout int // ms
		messages     []string
		expected     []string
		sleepTime    time.Duration
	}{
		{
			name:         "single message with timeout",
			batchSize:    5,
			batchTimeout: 50,
			messages:     []string{"test-message-1"},
			expected:     []string{"test-message-1"},
			sleepTime:    55 * time.Millisecond,
		},
		{
			name:         "batch size reached",
			batchSize:    2,
			batchTimeout: 1000,
			messages:     []string{"test-message-1", "test-message-2"},
			expected:     []string{"test-message-1\ntest-message-2"},
			sleepTime:    5 * time.Millisecond,
		},
		{
			name:         "large batch with timeout",
			batchSize:    10,
			batchTimeout: 50,
			messages:     []string{"test-message-1", "test-message-2", "test-message-3"},
			expected:     []string{"test-message-1\ntest-message-2\ntest-message-3"},
			sleepTime:    55 * time.Millisecond,
		},
		{
			name:         "multiple batches",
			batchSize:    2,
			batchTimeout: 50,
			messages:     []string{"msg1", "msg2", "msg3", "msg4", "msg5"},
			expected:     []string{"msg1\nmsg2", "msg3\nmsg4", "msg5"},
			sleepTime:    55 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			// Setup mocks
			conn := &MockInfluxDBConn{}
			request := &MockRestyRequest{}
			response := &resty.Response{}

			// Create a custom transport
			transport := &mockTransport{
				request: request,
			}

			// Create a client with our custom transport
			client := resty.New()
			client.SetTransport(transport)
			client.SetBaseURL("http://localhost:8086")

			// Setup mock expectations
			conn.On("Client").Return(client)
			request.On("SetContext", mock.Anything).Return(request)
			request.On("SetBody", mock.Anything).Return(request)
			request.On("Post", "").Return(response, nil)

			// Create sink with test configuration
			cfg := map[string]any{
				"batchSize":    tt.batchSize,
				"batchTimeout": tt.batchTimeout,
			}
			sink, err := NewInfluxDB(ctx, "test-sink", cfg, conn, nil)
			assert.NoError(t, err)

			// Start the sink
			err = sink.Start()
			assert.NoError(t, err)

			// Publish messages
			for _, msg := range tt.messages {
				sink.Publish(Msg{
					ThingId: "test-thing",
					Topic:   "test-topic",
					Payload: msg,
				})
			}

			// Wait for messages to be processed
			time.Sleep(tt.sleepTime)

			// Verify each expected batch was sent
			for _, expectedBody := range tt.expected {
				request.AssertCalled(t, "SetBody", expectedBody)
			}

			// Stop the sink
			err = sink.Stop()
			assert.NoError(t, err)
		})
	}
}

func TestInfluxDBErrorHandling(t *testing.T) {
	ctx := t.Context()

	// Setup mocks
	conn := &MockInfluxDBConn{}
	request := &MockRestyRequest{}
	response := &resty.Response{}

	client := resty.New()
	client.SetBaseURL("http://localhost:8086")
	client.SetTransport(&mockTransport{
		request: request,
	})

	conn.On("Client").Return(client)
	request.On("SetContext", mock.Anything).Return(&resty.Request{})
	request.On("SetBody", mock.Anything).Return(&resty.Request{})
	request.On("Post", "").Return(response, assert.AnError)

	// Create sink
	cfg := map[string]any{
		"batchSize":    1,
		"batchTimeout": 1 * time.Second,
	}
	sink, err := NewInfluxDB(ctx, "test-sink", cfg, conn, nil)
	assert.NoError(t, err)

	// Start the sink
	err = sink.Start()
	assert.NoError(t, err)

	// Send a message that will cause an error
	sink.Publish(Msg{
		ThingId: "test-thing",
		Topic:   "test-topic",
		Payload: "test-message",
	})

	// Wait for the error to be processed
	time.Sleep(15 * time.Millisecond)

	// Stop the sink
	err = sink.Stop()
	assert.NoError(t, err)
}

// mockTransport is a custom transport that intercepts HTTP requests
type mockTransport struct {
	request *MockRestyRequest
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	// Create a new reader for the body since we've already read it
	req.Body = io.NopCloser(bytes.NewReader(body))

	// Mock the request
	t.request.SetContext(req.Context())
	t.request.SetBody(string(body))
	resp, err := t.request.Post(req.URL.Path)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.IsError() {
		return resp.RawResponse, nil
	}

	// Return a successful response
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}, nil
}

// Add retry test cases
func TestInfluxDBRetry(t *testing.T) {
	tests := []struct {
		name          string
		batchSize     int
		batchTimeout  int // ms
		maxRetries    int
		retryInterval int // ms
		messages      []string
		errorCount    int // Number of initial requests that will fail
		expectSuccess bool
		sleepTime     time.Duration
	}{
		{
			name:          "retry success after two failures",
			batchSize:     2,
			batchTimeout:  50,
			maxRetries:    3,
			retryInterval: 10,
			messages:      []string{"msg1", "msg2"},
			errorCount:    2, // First two attempts fail, third succeeds
			expectSuccess: true,
			sleepTime:     100 * time.Millisecond,
		},
		{
			name:          "exceed max retries",
			batchSize:     2,
			batchTimeout:  50,
			maxRetries:    2,
			retryInterval: 10,
			messages:      []string{"msg3", "msg4"},
			errorCount:    3, // All retries fail
			expectSuccess: false,
			sleepTime:     100 * time.Millisecond,
		},
		{
			name:          "http error retry",
			batchSize:     1,
			batchTimeout:  50,
			maxRetries:    2,
			retryInterval: 10,
			messages:      []string{"msg5"},
			errorCount:    1, // First attempt returns HTTP error, second succeeds
			expectSuccess: true,
			sleepTime:     80 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()

			// Setup mocks
			conn := &MockInfluxDBConn{}
			request := NewMockRestyRequest()

			// Create a custom transport
			transport := &mockTransport{
				request: request,
			}

			// Create a client with our custom transport
			client := resty.New()
			client.SetTransport(transport)
			client.SetBaseURL("http://localhost:8086")

			// Setup mock expectations with retry behavior
			conn.On("Client").Return(client)
			request.On("SetContext", mock.Anything).Return(request)
			request.On("SetBody", mock.Anything).Return(request)

			// Mock the Post method with retry logic
			var response *resty.Response = &resty.Response{}
			request.On("Post", "").Run(func(args mock.Arguments) {
				callCount := request.incrementCallCount()
				slog.Debug("Mock Post call", "callCount", callCount, "errorCount", tt.errorCount)
				if callCount <= tt.errorCount {
					// Simulate failure cases
					*response = resty.Response{
						RawResponse: &http.Response{
							StatusCode: http.StatusBadGateway,
							Body:       io.NopCloser(bytes.NewReader([]byte("Error for testing on call: " + strconv.Itoa(callCount)))),
						},
					}
				} else {
					// After error count exceeded, return success
					*response = resty.Response{
						RawResponse: &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewReader([]byte(""))),
						},
					}
				}
			}).Return(response, nil)

			// Create sink with test configuration
			cfg := map[string]any{
				"batchSize":     tt.batchSize,
				"batchTimeout":  tt.batchTimeout,
				"maxRetries":    tt.maxRetries,
				"retryInterval": tt.retryInterval,
			}
			sink, err := NewInfluxDB(ctx, "test-sink", cfg, conn, nil)
			assert.NoError(t, err)

			// Start the sink
			err = sink.Start()
			assert.NoError(t, err)

			// Publish messages
			for _, msg := range tt.messages {
				sink.Publish(Msg{
					ThingId: "test-thing",
					Topic:   "test-topic",
					Payload: msg,
				})
			}

			// Wait for messages to be processed
			time.Sleep(tt.sleepTime)

			// Verify retry behavior
			actualCallCount := request.getCallCount()
			if tt.expectSuccess {
				assert.Equal(t, tt.errorCount+1, actualCallCount,
					"Should have retried exactly %d times before success", tt.errorCount)
			} else {
				assert.Equal(t, tt.maxRetries+1, actualCallCount,
					"Should have tried exactly %d times (initial + retries)", tt.maxRetries+1)
			}

			// Check the request body
			request.AssertCalled(t, "SetBody", strings.Join(tt.messages, "\n"))

			// Stop the sink
			err = sink.Stop()
			assert.NoError(t, err)
		})
	}
}
