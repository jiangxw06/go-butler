## golang项目管家

go-butler是一个golang微服务项目的脚手架类库。其中的common包整合了golang微服务项目常用的多种资源，data包实现了一个简单好用的缓存代理框架。


###common包

common包实现的主要特性包括：
- 基于配置的声明式资源管理，托管了资源的初始化和优雅关闭
- 基于zap的开箱即用的日志框架
- 增强了context，注入了requestID、耗时等字段，方便tracing和日志
- 基于Gin的Http服务器，封装了panic recover、日志、打点、render等功能
- 基于etcd的GRPC服务注册与发现
- 增强了grpc客户端/服务器，封装了保活、tracing、日志、超时、反射等功能
- 基于sarama的kafka消费者/生产者，封装了tracing、日志等功能
- 集成了redis、mysql和rocketmq等资源的管理

go-butler用基于toml的配置文件实现了日志、监控、数据库、消息队列等资源的声明式管理。配置文件默认路径是"./config/app.toml"，可以使用"config_file"的flag或者调用common.ReadConfigFile(configFilePath)来指定配置文件的目录。也可以用common.ReadConfig(configStr)直接读入配置项，通常用于测试或临时的项目中。

```js
#配置文件示例
[app]
deploy  = "local"
appName = "myproject"

[log]
MaxSize = 50 # megabytes
MaxBackups = 5
MaxAge = 10 #days

#etcd用于服务注册与发现
[etcd]
environment = "dev"
addrs = ["10.18.8.240:4013", "10.18.8.241:4013", "10.18.8.242:4013"]

[service]
#依赖的其他服务必须显式声明
#dependencies = ["upstreamGRPC1", "upstreamGRPC2", "upstreamGRPC3"]
#下面是本应用暴露的grpc服务，可以有多个，格式为service.register.{serviceName}
[service.register.mygrpcserver]
port = 8902

[redis]
poolsize = 1000
pooltimeout = 500
[redis.clusters.default]
addrs = ["10.1.2.3:6379","10.1.2.4:6379","10.1.2.5:6379"]
pass = "f798bb129840404d8e1a960134da69fe"
[redis.clusters.snowflake]
appid = ["10.1.2.3:6379","10.1.2.4:6379","10.1.2.5:6379"]
pass = "f798bb129840404d8e1a960134da69fe"

[http]
[http.default]
http_port = 9000
[http.prometheus]
http_port = 9001

[db]
maxIdle = 5
maxConn = 30
[db.connects]
ro = "user_ro:password1@tcp(db.mycompany.com:3306)/myschema?parseTime=true&loc=Local&charset=utf8mb4"
rw = "user_rw:password2@tcp(db.mycompany.com:3306)/myschema?parseTime=true&loc=Local&charset=utf8mb4"

[kafka]
[kafka.clusters]
cluster1 = [
"kafkabroker1.mycompnay.com:6667",
"kafkabroker2.mycompnay.com:6667",
"kafkabroker3.mycompnay.com:6667"
]
[kafka.topics.mytopic1]
cluster = "cluster1"
topic = "mykafkatopic1"
group = "mykafkagroup1"
```

go-butler将http服务器、grpc客户端/服务器、mysql客户端、redis客户端、kafka消费者/生产者等资源封装成基础服务，托管了资源的初始化和优雅关闭，用户只须配置和实现服务逻辑，就可以像搭积木一样方便地添加新的业务，简化了资源的管理。
以下是一个启动一个测试http服务器的简单示例。

```js
package main

import (
    "github.com/gin-gonic/gin"
"github.com/jiangxw06/go-butler"
)

func main() {
    common.ReadConfig(`
[app]
deploy="local"
appName="butler_example"

[http]
[http.main]
http_port=8090
`)
    engine := common.NewGinEngine()
    common.GinHandle(engine.Group("/api"),"GET","/test",common.FullLogContent, func(ctx *gin.Context) {
        common.SetHttpBody(ctx, struct {
            Status int
            Data   string
        }{Status:200,Data:"test"})
    })
    server := common.MustGetHttpServer("main")
    server.HttpServer.Handler = engine
    common.StartHttpServer(server)
    common.WaitClose()
}
```

###data包

data包实现了数据缓存及持久化相关的模式和框架，包括：
- 基于publish/subscribe模式的db数据项变更框架
- 实现了cache aside模式的redis缓存代理框架
- 基于redis和lua脚本的分布式锁实现
- 基于snowflake算法的分布式ID生成功能

