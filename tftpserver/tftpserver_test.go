package tftpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/inngest/shoelaces/log"
	tftp "github.com/pin/tftp/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTFTPFailureHookDowngradesClientAbortDuringOptionNegotiation(t *testing.T) {
	var output bytes.Buffer
	hook := tftpHook{
		logger: log.MakeLogger(&output, log.WithHandler(log.HandlerJSON), log.WithLevel(log.LevelDebug)),
	}

	hook.OnFailure(tftp.TransferStats{
		RemoteAddr:     net.ParseIP("192.0.2.10"),
		Filename:       "snponly.efi",
		Mode:           "octet",
		Duration:       3 * time.Millisecond,
		DatagramsSent:  1,
		DatagramsAcked: 0,
	}, errors.New("sending block 0: code=8, error: User aborted the transfer\x00"))

	record := decodeJSONLogRecord(t, output.Bytes())
	assert.Equal(t, "DEBUG", record["level"])
	assert.Equal(t, "TFTP transfer aborted by client during option negotiation", record["msg"])
	assert.Equal(t, "snponly.efi", record["filename"])
	assert.Equal(t, float64(1), record["datagrams_sent"])
	assert.Equal(t, float64(0), record["datagrams_acked"])
}

func TestTFTPFailureHookWarnsForClientAbortAfterDataStarts(t *testing.T) {
	var output bytes.Buffer
	hook := tftpHook{
		logger: log.MakeLogger(&output, log.WithHandler(log.HandlerJSON), log.WithLevel(log.LevelDebug)),
	}

	hook.OnFailure(tftp.TransferStats{
		RemoteAddr:     net.ParseIP("192.0.2.10"),
		Filename:       "snponly.efi",
		Mode:           "octet",
		Duration:       42 * time.Millisecond,
		DatagramsSent:  12,
		DatagramsAcked: 11,
	}, errors.New("sending block 12: code=8, error: User aborted the transfer\x00"))

	record := decodeJSONLogRecord(t, output.Bytes())
	assert.Equal(t, "WARN", record["level"])
	assert.Equal(t, "TFTP transfer failed", record["msg"])
	assert.Equal(t, float64(12), record["datagrams_sent"])
	assert.Equal(t, float64(11), record["datagrams_acked"])
}

func TestTFTPFailureClassifierRequiresBlockZeroAbort(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "block zero user abort",
			err:  errors.New("sending block 0: code=8, error: User aborted the transfer\x00"),
			want: true,
		},
		{
			name: "later user abort",
			err:  errors.New("sending block 2: code=8, error: User aborted the transfer\x00"),
			want: false,
		},
		{
			name: "block zero different code",
			err:  errors.New("sending block 0: code=1, error: not found"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTFTPOptionNegotiationAbort(tt.err))
		})
	}
}

func decodeJSONLogRecord(t *testing.T, line []byte) map[string]any {
	t.Helper()

	var record map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(line), &record))
	return record
}
