package log

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerDevHandlerUsesMsgAttribute(t *testing.T) {
	var output bytes.Buffer
	logger := MakeLogger(&output, WithHandler(HandlerDev))

	logger.Info("rendered", "component", "template", "template", "boot.ipxe")

	assert.Contains(t, output.String(), "INF rendered")
	assert.Contains(t, output.String(), "component=template")
	assert.Contains(t, output.String(), "template=boot.ipxe")
	assert.NotContains(t, output.String(), "msg=")
}

func TestLoggerHonorsLevel(t *testing.T) {
	var output bytes.Buffer
	logger := MakeLogger(&output, WithHandler(HandlerText), WithLevel(LevelInfo))

	logger.Debug("hidden")
	logger.Info("visible")

	assert.NotContains(t, output.String(), "hidden")
	assert.Contains(t, output.String(), "visible")
}

func TestDebugLevelEnablesDebugLogs(t *testing.T) {
	var output bytes.Buffer
	logger := MakeLogger(&output, WithHandler(HandlerText), WithLevel(LevelDebug))

	logger.Debug("visible")

	assert.Contains(t, output.String(), "visible")
}

func TestLoggerDevHandlerFormatsAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := MakeLogger(&output, WithHandler(HandlerDev))

	logger.Info("dir list failed", "component", "ipxescript", "dir", "/tmp/data", "err", "missing")

	assert.Contains(t, output.String(), `INF dir list failed`)
	assert.Contains(t, output.String(), `component=ipxescript`)
	assert.Contains(t, output.String(), `dir=/tmp/data`)
	assert.Contains(t, output.String(), `err=missing`)
}

func TestLoggerDevHandlerFormatsTypedNilAttributes(t *testing.T) {
	var output bytes.Buffer
	logger := MakeLogger(&output, WithHandler(HandlerDev), WithLevel(LevelDebug))

	var value *panicStringer
	logger.Debug("manual action", "script", value)

	assert.Contains(t, output.String(), `DBG manual action`)
	assert.Contains(t, output.String(), `script=<nil>`)
}

type panicStringer struct{}

func (*panicStringer) String() string {
	panic("nil stringer should not be called")
}
