package url

import (
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func GetURL(name string) (string, error) {
	key := "url." + name
	if !viper.IsSet(key) {
		return "", errors.New("url not config")
	}
	return viper.GetString(key), nil
}

func MustGetURL(name string) string {
	url, err := GetURL(name)
	if err != nil {
		panic(err)
	}
	return url
}
