package nats

import "ruff.io/tio/config"

// FromConfig maps a tio NATS config block to the bridge Config. The bridge
// reads superUsers separately (New), so only the fields it uses are carried.
func FromConfig(cfg config.NatsConfig) Config {
	return Config{
		Enabled:      cfg.Enabled,
		Mode:         cfg.Mode,
		URL:          cfg.URL,
		Server:       cfg.Server,
		AppClient:    cfg.AppClient,
		SystemClient: cfg.SystemClient,
		JetStream: JetStreamOptions{
			Enabled:       cfg.JetStream.Enabled,
			StoreDir:      cfg.JetStream.StoreDir,
			MaxMemory:     cfg.JetStream.MaxMemory,
			MaxStore:      cfg.JetStream.MaxStore,
			MaxMsgs:       cfg.JetStream.MaxMsgs,
			MaxBytes:      cfg.JetStream.MaxBytes,
			MaxAgeSeconds: cfg.JetStream.MaxAgeSeconds,
		},
	}
}
