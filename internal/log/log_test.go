package log

import (
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestAccessLogger(t *testing.T) {
	AccessLogger().Info("access log info")

	env.CloseComponents()
	CloseSysLogger()
}

func TestSysLogger(t *testing.T) {
	SysLogger().Info("sys log info")

	env.CloseComponents()
	CloseSysLogger()
}

func TestLogConfig(t *testing.T) {
	env.MergeConfig(`
[log]
    maxSize=50 # megabytes
    maxBackups=5
    maxAge=10 #days
    localTime=true
    #日志级别有"debug","info","error"
    consoleLogLevel= "info" #ConsoleLogLevel表示输出到std.Out的日志级别
`)
	AccessLogger().Info("access log info")
	SysLogger().Info("sys log info")

	env.CloseComponents()
	CloseSysLogger()
}

func TestDefaultLogFieldsWithNoConfig(t *testing.T) {
	assert.Equal(t, len(DefaultAccessLogFields()), 0)

	env.CloseComponents()
	CloseSysLogger()
}

func TestDefaultLogFieldsWithConfig(t *testing.T) {
	env.MergeConfig(`
[log]
`)
	fileds := DefaultAccessLogFields()
	assert.Equal(t, len(fileds), 2)

	env.CloseComponents()
	CloseSysLogger()
}
