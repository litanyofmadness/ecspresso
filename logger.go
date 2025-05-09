package ecspresso

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/fujiwara/sloghandler"
)

var (
	logLevel           = new(slog.LevelVar)
	commonLogger       = slog.New(sloghandler.NewLogHandler(os.Stderr, slogHandlerOptions))
	slogHandlerOptions = &sloghandler.HandlerOptions{
		Color: true,
		HandlerOptions: slog.HandlerOptions{
			Level: logLevel,
		},
	}
)

func newLogger() *slog.Logger {
	return slog.New(sloghandler.NewLogHandler(io.Discard, slogHandlerOptions))
}

func Log(f string, v ...interface{}) {
	msg := fmt.Sprintf(f, v...)
	commonLogger.Info(msg)
}

func (d *App) Log(f string, v ...interface{}) {
	msg := fmt.Sprintf(f, v...)
	d.logger.Info(msg)
}

func (d *App) LogJSON(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		d.logger.Warn("failed to marshal json", "error", err.Error())
		return
	}
	fmt.Fprintln(os.Stderr, string(b)) // TODO
}
