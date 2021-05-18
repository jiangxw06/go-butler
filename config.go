package common

import (
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/spf13/viper"
	"strconv"
)

type appConfig struct{}

var (
	AppConfig = &appConfig{}
)

func (b *appConfig) String(key string) (string, bool) {
	return viper.GetString(key), viper.IsSet(key)
}

func (b *appConfig) Strings(key string) ([]string, bool) {
	return viper.GetStringSlice(key), viper.IsSet(key)
}

func (b *appConfig) Int(key string) (int, bool) {
	return viper.GetInt(key), viper.IsSet(key)
}

func (b *appConfig) IntSlice(key string) ([]int, bool) {
	return viper.GetIntSlice(key), viper.IsSet(key)
}

func (b *appConfig) Int64(key string) (int64, bool) {
	return viper.GetInt64(key), viper.IsSet(key)
}

func (b *appConfig) Bool(key string) (bool, bool) {
	return viper.GetBool(key), viper.IsSet(key)
}

func (b *appConfig) Float(key string) (float64, bool) {
	return viper.GetFloat64(key), viper.IsSet(key)
}

func (b *appConfig) FloatSlice(key string) ([]float64, bool) {
	ok := viper.IsSet(key)
	strs := viper.GetStringSlice(key)
	//strs, ok := viper.GetStringSlice(key), viper.IsSet(key)
	if !ok {
		return nil, false
	}
	var res []float64
	for _, str := range strs {
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return nil, false
		}
		res = append(res, v)
	}
	return res, true
}

func (b *appConfig) DefaultString(key string, defaultVal string) string {
	if v, ok := b.String(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultStrings(key string, defaultVal []string) []string {
	if v, ok := b.Strings(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultInt(key string, defaultVal int) int {
	if v, ok := b.Int(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultIntSlice(key string, defaultVal []int) []int {
	if v, ok := b.IntSlice(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultInt64(key string, defaultVal int64) int64 {
	if v, ok := b.Int64(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultBool(key string, defaultVal bool) bool {
	if v, ok := b.Bool(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultFloat(key string, defaultVal float64) float64 {
	if v, ok := b.Float(key); ok {
		return v
	}
	return defaultVal
}

func (b *appConfig) DefaultFloatSlice(key string, defaultVal []float64) []float64 {
	if v, ok := b.FloatSlice(key); ok {
		return v
	}
	return defaultVal
}
