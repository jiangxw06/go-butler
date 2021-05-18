package log

import (
	"fmt"
	"github.com/jiangxw06/go-butler/internal/env"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"testing"
)

func TestDebugMode(t *testing.T) {
	logs := NewDebugModeLogs()
	logger := NewDebugModeLogger(logs)
	logger.Error("log 1", zap.Field{
		Key:    "key1",
		Type:   zapcore.StringType,
		String: "value1",
	})
	logger.Error("log 2", zap.Field{
		Key:     "key2",
		Type:    zapcore.DurationType,
		String:  "value2",
		Integer: 10000,
	})
	fmt.Println(logs.All())
	env.CloseComponents()
}
