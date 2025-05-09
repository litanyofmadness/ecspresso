package ecspresso_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/kayac/ecspresso/v2"
)

var logLevels = []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

func TestCommonLogger(t *testing.T) {
	for _, format := range []string{"text", "json"} {
		for _, level := range logLevels {
			b := new(bytes.Buffer)
			ecspresso.SetLogFormat(format)
			logger := ecspresso.NewLogger(b)
			ecspresso.LogLevel.Set(level)
			ecspresso.SetLogger(logger)

			ecspresso.LogDebug("test %s", level)
			ecspresso.LogInfo("test %s", level)
			ecspresso.LogWarn("test %s", level)
			ecspresso.LogError("test %s", level)
			t.Log(b.String())
		}
	}
}

func TestLogger(t *testing.T) {
	app := &ecspresso.App{}
	for _, format := range []string{"text", "json"} {
		for _, level := range logLevels {
			b := new(bytes.Buffer)
			ecspresso.SetLogFormat(format)
			logger := ecspresso.NewLogger(b)
			ecspresso.LogLevel.Set(level)
			app.SetLogger(logger)

			app.LogDebug("test %s", "test")
			app.LogInfo("test %s", "test")
			app.LogWarn("test %s", "test")
			app.LogError("test %s", "test")
			t.Log(b.String())
		}
	}
}
