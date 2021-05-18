package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
)

type (
	debugModeCore struct {
		zapcore.LevelEnabler
		enc  zapcore.Encoder
		logs *DebugModeLogs
	}
	DebugModeLogs struct {
		mu   sync.Mutex
		logs []string
	}
)

var (
	_ zapcore.Core = new(debugModeCore)
)

func NewDebugModeLogger(logs *DebugModeLogs) *zap.Logger {
	logOnce.Do(loadLogConfig)
	if logs == nil {
		panic("can not happen")
	}
	core := newDebugModeCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		logs,
		zap.NewAtomicLevelAt(zapcore.DebugLevel),
	)
	return zap.New(core)
}

// NewDebugModeCore
func newDebugModeCore(enc zapcore.Encoder, logs *DebugModeLogs, enab zapcore.LevelEnabler) zapcore.Core {
	return &debugModeCore{
		LevelEnabler: enab,
		enc:          enc,
		logs:         logs,
	}
}

func NewDebugModeLogs() *DebugModeLogs {
	return &DebugModeLogs{
		logs: make([]string, 0),
	}
}

func (c *debugModeCore) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	for i := range fields {
		fields[i].AddTo(clone.enc)
	}
	return clone
}

func (c *debugModeCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *debugModeCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	buf, err := c.enc.EncodeEntry(ent, fields) //把日志编码为bytes
	if err != nil {
		return err
	}
	l := string(buf.Bytes())
	c.logs.Add(l) //把日志添加到logs中
	buf.Free()
	return nil
}

func (c *debugModeCore) Sync() error {
	return nil
}

func (c *debugModeCore) clone() *debugModeCore {
	return &debugModeCore{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc.Clone(),
		logs:         c.logs,
	}
}

// Len returns the number of items in the collection.
func (o *DebugModeLogs) Len() int {
	o.mu.Lock()
	n := len(o.logs)
	o.mu.Unlock()
	return n
}

// All returns a copy of all the observed logs.
func (o *DebugModeLogs) All() []string {
	o.mu.Lock()
	ret := make([]string, len(o.logs))
	for i := range o.logs {
		ret[i] = o.logs[i]
	}
	o.mu.Unlock()
	return ret
}

func (o *DebugModeLogs) Add(log string) {
	o.mu.Lock()
	o.logs = append(o.logs, log)
	o.mu.Unlock()
}
