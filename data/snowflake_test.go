package data

import (
	"github.com/jiangxw06/go-butler"
	"testing"
)

func TestSnowflakeID(t *testing.T) {
	time, ts, workerID, seq := ParseSnowflakeID(576187175373385728)
	common.Logger(common.SetupContext(nil)).Infow("parse", "time", time, "ts", ts, "workerID", workerID, "seq", seq)
}
