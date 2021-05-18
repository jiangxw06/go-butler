package common

import (
	"github.com/jiangxw06/go-butler/internal/env"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/magiconair/properties/assert"
	"math"
	"testing"
)

func TestViper(t *testing.T) {
	env.MergeConfig(`
k1="1"
k2=[1,2,3]
k3=1.23
k4=true
k5=2
[section1]
	k6=3.01
`)
	v, ok := AppConfig.Strings("k1")
	assert.Equal(t, v, []string{"1"})
	assert.Equal(t, ok, true)
	v2, ok := AppConfig.String("notexist")
	assert.Equal(t, v2, "")
	assert.Equal(t, ok, false)
	v3, ok := AppConfig.Float("k3")
	assert.Equal(t, v3, 1.23)
	v4, ok := AppConfig.Bool("k4")
	assert.Equal(t, v4, true)
	v5, ok := AppConfig.Int("k5")
	assert.Equal(t, v5, 2)
	v6, _ := AppConfig.Float("section1.k6")
	assert.Equal(t, v6, 3.01)
}

func TestFloatSlice(t *testing.T) {
	//必须是2.0而不能是2
	env.MergeConfig(`
k1 = [1.2,2.0,-3.0,+inf]
`)
	//v, ok := AppConfig.Strings("k1")
	v, ok := AppConfig.FloatSlice("k1")
	assert.Equal(t, ok, true)
	assert.Equal(t, v, []float64{1.2, 2.0, -3, math.Inf(1)})
}
