package environment

import (
	"time"
)

// TFTPConfig carries runtime settings for the embedded TFTP server.
type TFTPConfig struct {
	// Enabled starts the embedded TFTP server alongside the HTTP server.
	Enabled bool
	// Addr is the UDP listen address for TFTP requests.
	Addr string
	// Root is the directory served over TFTP.
	Root string
	// Readonly rejects client upload attempts when true.
	Readonly bool
	// Timeout bounds each TFTP request so stalled clients do not hold resources indefinitely.
	Timeout time.Duration
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
