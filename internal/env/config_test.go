package env

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestMergeConfigFile(t *testing.T) {
	MergeConfigFile("./testdata/app.toml")
	assert.Equal(t, GetAppName(), "config_path_test")
	assert.Equal(t, IsLocal(), true)
}

func TestMergeConfig(t *testing.T) {
	MergeConfig(`
[app]
appName="test_merge_config"
`)
	assert.Equal(t, GetAppName(), "test_merge_config")
}
