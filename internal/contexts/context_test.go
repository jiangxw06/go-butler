package contexts

import (
	"context"
	"github.com/jiangxw06/go-butler/internal/env"
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	Logger(nil).Infow("log 1", "key1", "v1")
	time.Sleep(time.Second)
	Logger(nil).Info("log2")
	ctx2 := SetupContext(context.Background())
	Logger(ctx2).Infow("log 3", "key1", "v1")
	time.Sleep(time.Second)
	Logger(ctx2).Info("log4")
	env.CloseComponents()
}
