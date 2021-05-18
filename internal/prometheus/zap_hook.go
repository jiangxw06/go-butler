package prometheus

import (
	"go.uber.org/zap/zapcore"
)

func ZapHook(logType string) func(zapcore.Entry) error {
	return func(e zapcore.Entry) error {
		if e.Level < zapcore.ErrorLevel {
			return nil
		}
		ErrorCounter(logType).Inc()
		return nil
	}
}
