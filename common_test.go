package common

import (
	"testing"
)

func TestPProf(t *testing.T) {
	ReadConfig(`
[http]
    [http.default]
        http_port = 9000
    [http.prometheus]
        http_port = 9001
`)
	StartPromServer()
	WaitClose()
}
