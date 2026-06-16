package environment

import (
	"time"
)

// TFTPConfig carries runtime settings for the embedded TFTP server.
type TFTPConfig struct {
	Enabled  bool
	Addr     string
	Root     string
	Readonly bool
	Timeout  time.Duration
}

// DefaultTFTPConfig returns the defaults used by the CLI and runtime.
func DefaultTFTPConfig() TFTPConfig {
	return TFTPConfig{
		Enabled:  false,
		Addr:     ":69",
		Root:     "./tftp",
		Readonly: true,
		Timeout:  5 * time.Second,
	}
}
