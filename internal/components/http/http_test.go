package http

import (
	"context"
	"flag"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/magiconair/properties/assert"
	"os"
	"testing"
)

var (
	ctx context.Context
)

func TestMain(m *testing.M) {
	env.MergeConfig(`
[http]
    [http.default]
        http_port = 9000
    [http.prometheus]
        http_port = 9001
`)

	flag.Parse()
	ctx = contexts.SetupContext(nil)
	exitCode := m.Run()

	env.CloseComponents()
	// 退出
	os.Exit(exitCode)
}

func TestLoadHttpConfig(t *testing.T) {
	initHttpConfig()
	assert.Equal(t, httpConf["default"].HttpPort, 9000)
}

type (
	HTTPResp struct {
		StatusText string `json:"statusText"`
		Message    string `json:"message"`
		Status     int    `json:"status"`
	}
)
