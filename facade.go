package common

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/go-resty/resty/v2"
	gin2 "github.com/jiangxw06/go-butler/internal/components/gin"
	grpc2 "github.com/jiangxw06/go-butler/internal/components/grpc"
	http2 "github.com/jiangxw06/go-butler/internal/components/http"
	"github.com/jiangxw06/go-butler/internal/components/kafka"
	"github.com/jiangxw06/go-butler/internal/components/mysql"
	redis2 "github.com/jiangxw06/go-butler/internal/components/redis"
	"github.com/jiangxw06/go-butler/internal/components/rocketmq"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gorm.io/gorm"
	"time"
)

//CopyContext 返回一个新的context，但复制了参数ctx的requestID,startTime等字段
func CopyContext(ctx context.Context) context.Context {
	return contexts.CopyContext(ctx)
}

//Logger 返回访问日志的logger
//ctx可以是grpc或gin.Context或kafka等本框架生成的context
func Logger(ctx context.Context) *zap.SugaredLogger {
	return contexts.Logger(ctx)
}

//SetupContext 把访问日志的logger注入ctx中
//在本方法中生成了RequestID
//requestID暂时只在本进程内有效，不会传到grpc下游
//还生成了访问开始时间
func SetupContext(ctx context.Context) context.Context {
	return contexts.SetupContext(ctx)
}

//SysLogger 返回系统日志的logger
//SysLogger用于系统初始化等与外部访问无关的日志的记录
func SysLogger() *zap.SugaredLogger {
	return log.SysLogger()
}

//GrpcClientConn 返回框架增强过的grpc客户端连接
//serviceName 是app.toml中配置的依赖服务名称，必须在service.dependencies中声明
func GrpcClientConn(serviceName string, unaryInterceptors ...grpc.UnaryClientInterceptor) (*grpc.ClientConn, error) {
	return grpc2.GetGrpcClientConn(serviceName, unaryInterceptors...)
}

//MustGrpcClientConn 与GrpcClientConn功能相同，但出错时会panic
func MustGrpcClientConn(serviceName string, unaryInterceptors ...grpc.UnaryClientInterceptor) *grpc.ClientConn {
	return grpc2.MustGetGrpcClientConn(serviceName, unaryInterceptors...)
}

//NewGrpcServer 返回框架增强过的grpc服务器，提供了日志、监控等能力
func NewGrpcServer(f func(server *grpc.Server), opt ...grpc.UnaryServerInterceptor) *grpc.Server {
	return grpc2.NewServer(f, opt...)
}

//StartGrpcServer 启动一个grpc服务器
//serviceName 是app.toml中配置的注册服务名称，路径为service.register.<serviceName>
func StartGrpcServer(serviceName string, server *grpc.Server) {
	go grpc2.StartGrpcServer(serviceName, server)
}

//HttpRequest 返回一个增强过的http客户端
func HttpRequest(ctx context.Context) *resty.Request {
	return http2.Request(ctx)
}

//HttpRequest 返回一个增强过的有超时设置的http客户端
func HttpRequestWithTimeout(ctx context.Context, timeout time.Duration) (*resty.Request, context.CancelFunc) {
	return http2.RequestWithTimeout(ctx, timeout)
}

//NewGinEngine 返回一个增强过的gin引擎，提供了context、日志、panic恢复、监控等能力
func NewGinEngine() *gin.Engine {
	return gin2.NewEngine()
}

const (
	NoneLogContent LogContent = iota
	MetadataLogContent
	FullLogContent
)

type (
	LogContent = int
)

//GinHandleAndLog 封装了Gin的RouterGroup.Handle方法，并增加配置日志级别的功能
func GinHandle(group *gin.RouterGroup, httpMethod, relativePath string, logContent LogContent, handlers ...gin.HandlerFunc) gin.IRoutes {
	return gin2.HandleAndLog(group, httpMethod, relativePath, logContent, handlers...)
}

//GinHandleAndLog 封装了Gin的RouterGroup.Any方法，并增加配置日志级别的功能
func GinAny(group *gin.RouterGroup, relativePath string, logContent LogContent, handlers ...gin.HandlerFunc) gin.IRoutes {
	return gin2.HandleAnyAndLog(group, relativePath, logContent, handlers...)
}

//SetHttpBody 设置http请求的json格式的响应body
//obj 是struct或其他可以json序列化的对象
func SetHttpBody(ctx *gin.Context, obj interface{}) {
	gin2.SetBody(ctx, obj)
}

//GetHttpServer 返回app.toml中配置的http服务器，路径为http.<name>
func GetHttpServer(name string) (*http2.Server, error) {
	return http2.GetServer(name)
}

//MustGetHttpServer 返回app.toml中配置的http服务器，路径为http.<name>
//若配置不存在，则panic
func MustGetHttpServer(name string) *http2.Server {
	return http2.MustGetServer(name)
}

//StartHttpServer 启动http服务器
//调用方应该在启动服务之前设置server.HttpServer.Handler为框架生成的ginEngine
func StartHttpServer(server *http2.Server) {
	go http2.StartHttpServer(server)
}

//StartRocketMQConsumer 启动RocketMQ的消费者
//name 为app.toml中配置的topic名称，路径为kafka.topics.<name>
//consumeFunc 为收到消息后回调的消费方法
func StartRocketMQConsumer(name string, consumeFunc func(ctx context.Context, msg []byte) bool) {
	go rocketmq.StartRocketMQConsumer(name, consumeFunc)
}

//SendRocketMQMessage 发送RocketMQ消息
//name 为app.toml中配置的topic名称，路径为kafka.topics.<name>
//key 为消息的key
//data 为消息的data
func SendRocketMQMessage(ctx context.Context, name, key string, data []byte) (err error) {
	return rocketmq.SendRocketMQMessage(ctx, name, key, data)
}

//SendKafkaMessage 发送Kafka消息
//name 为app.toml中配置的topic名称，路径为rocketmq.topics.<name>
//key 为消息的key
//data 为消息的data
func SendKafkaMessage(ctx context.Context, name, key string, data []byte) (err error) {
	return kafka.SendKafkaMessage(ctx, name, key, data)
}

//StartKafkaConsumer 启动Kafka消费者
//name 为app.toml中配置的topic名称，路径为rocketmq.topics.<name>
//consumeFunc 为收到消息后回调的消费方法
func StartKafkaConsumer(name string, consumeFunc func(ctx context.Context, msg []byte)) {
	go kafka.StartKafkaConsumer(name, consumeFunc)
}

//GetDB 获得app.toml中配置的gorm类型的db
//name 路径为db.connects.<name>
//replicaNames 用于额外指定数据库的只读库，路径也是db.connects.<name>，select等读操作会随机分配到所有库上面
func GetDB(name string, replicaNames ...string) (*gorm.DB, error) {
	return mysql.GetGorm(name, replicaNames...)
}

//GetDB 获得app.toml中配置的gorm类型的db
//name 路径为db.connects.<name>
//replicaNames 用于额外指定数据库的只读库，路径也是db.connects.<name>，select等读操作会随机分配到所有库上面
//若配置不存在则panic
func MustGetDB(name string, replicaNames ...string) *gorm.DB {
	return mysql.MustGetGorm(name, replicaNames...)
}

//GetSqlxDB 获得app.toml中配置的sqlx类型的db
//name 路径为db.connects.<name>
func GetSqlxDB(name string) (*sqlx.DB, error) {
	return mysql.GetSqlxDB(name)
}

//GetSqlxDB 获得app.toml中配置的sqlx类型的db
//name 路径为db.connects.<name>
//若配置不存在则panic
func MustGetSqlxDB(name string) *sqlx.DB {
	return mysql.MustGetSqlxDB(name)
}

//GetRedisClient 获得app.toml中配置的redis客户端
//key 路径为redis.clusters.<key>
func GetRedisClient(key string) (redis.UniversalClient, error) {
	return redis2.GetRedisClient(key)
}

//GetRedisClient 获得app.toml中配置的redis客户端
//key 路径为redis.clusters.<key>
//若配置不存在则panic
func MustGetRedisClient(key string) redis.UniversalClient {
	return redis2.MustGetRedisClient(key)
}

func IsProd() bool {
	return env.IsProd()
}

func IsTest() bool {
	return env.IsTest()
}

func IsLocal() bool {
	return env.IsLocal()
}

func GetIP() string {
	return env.GetIP()
}

func GetAppName() string {
	return env.GetAppName()
}
