package conf

import (
	"fmt"
	"time"
)

// AuditConfig controls the bounded, best-effort operation audit pipeline.
type AuditConfig struct {
	QueueCapacity int           `mapstructure:"queue_capacity"`
	BatchSize     int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	WriteTimeout  time.Duration `mapstructure:"write_timeout"`
}

// WithDefaults fills omitted audit settings without changing explicitly invalid values.
func (c AuditConfig) WithDefaults() AuditConfig {
	if c.QueueCapacity == 0 {
		c.QueueCapacity = 1024
	}
	if c.BatchSize == 0 {
		c.BatchSize = 50
	}
	if c.FlushInterval == 0 {
		c.FlushInterval = time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 2 * time.Second
	}
	return c
}

// Validate checks resource bounds before starting any background workers.
func (c AuditConfig) Validate() error {
	c = c.WithDefaults()
	if c.QueueCapacity < 1 || c.QueueCapacity > 65536 {
		return fmt.Errorf("audit.queue_capacity must be between 1 and 65536")
	}
	if c.BatchSize < 1 || c.BatchSize > c.QueueCapacity || c.BatchSize > 1000 {
		return fmt.Errorf("audit.batch_size must be between 1 and min(queue_capacity, 1000)")
	}
	if c.FlushInterval <= 0 || c.FlushInterval > time.Minute {
		return fmt.Errorf("audit.flush_interval must be positive and at most 1m")
	}
	if c.WriteTimeout <= 0 || c.WriteTimeout > 10*time.Second {
		return fmt.Errorf("audit.write_timeout must be positive and at most 10s")
	}
	return nil
}
