package env

import (
	"fmt"
	"github.com/spf13/viper"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultIP = "0.0.0.0"
)

type (
	appInfoConfig struct {
		Deploy  string `mapstructure:",omitempty"`
		AppName string `mapstructure:",omitempty"`
		IP      string
	}
)

var (
	envOnce sync.Once
	appConf = appInfoConfig{
		AppName: "no_app_name",
		Deploy:  "local",
		IP:      defaultIP,
	}
)

func loadConfig() {
	if viper.IsSet("app") {
		if err := viper.UnmarshalKey("app", &appConf); err != nil {
			panic(fmt.Errorf("unable to decode into struct：  %s \n", err))
		}
		if appConf.Deploy != "local" && appConf.Deploy != "test" && appConf.Deploy != "online" && appConf.Deploy != "prod" {
			panic("app.deploy must be local or test or online/prod")
		}
		appConf.AppName = strings.ReplaceAll(appConf.AppName, "-", "_")
	}

	//DomeOS上pod的ip环境变量名：MY_POD_IP，如：	172.25.208.3
	//DomeOS上node的ip环境变量名：MY_NODE_IP，如：	10.18.9.51
	if v := os.Getenv("MY_POD_IP"); v != "" {
		appConf.IP = v
	} else {
		appConf.IP = getOutBoundIP()
	}

}

func getIPv4() string {
	addrs, err := net.InterfaceAddrs()

	if err == nil {
		for _, address := range addrs {
			// 检查ip地址判断是否回环地址
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
	}

	return defaultIP
}

func getOutBoundIP() (ip string) {
	conn, err := net.DialTimeout("udp", "8.8.8.8:53", 3*time.Second)
	if err != nil {
		fmt.Printf("getOutBoundIP error: %v\n", err)
		return defaultIP
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = strings.Split(localAddr.String(), ":")[0]
	return
}

func IsProd() bool {
	envOnce.Do(loadConfig)
	return appConf.Deploy == "prod" || appConf.Deploy == "online"
}

func IsTest() bool {
	envOnce.Do(loadConfig)
	return appConf.Deploy == "test"
}

func IsLocal() bool {
	envOnce.Do(loadConfig)
	return appConf.Deploy == "local"
}

func GetIP() string {
	envOnce.Do(loadConfig)
	return appConf.IP
}

func GetAppName() string {
	envOnce.Do(loadConfig)
	return appConf.AppName
}

func HasConfigFile() bool {
	envOnce.Do(loadConfig)
	return false
}
