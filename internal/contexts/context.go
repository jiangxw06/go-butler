package contexts

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jiangxw06/go-butler/internal/log"
	"time"

	"github.com/teris-io/shortid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	//ctxStoreMarker struct{}
	ctxStore struct {
		logger        *zap.SugaredLogger
		startTime     time.Time
		requestID     string
		debugModeLogs *log.DebugModeLogs
		logFields     []zapcore.Field
		tags          map[string]interface{}
	}
)

var (
	ctxStoreMarkerKey = "ctxStoreMarker" //为了支持gin.Context，将key改成string类型
	//ctxStoreMarkerKey = &ctxStoreMarker{}
)

// AddLogFields 注入本次访问的相关日志字段
//应该传入http请求的Path/grpc请求的Method/kafka消息的topic等代表caller类型的信息
//e.g. fields{"callType":"http","callName":"/repos_example/v3/hot"}

func AddLogFields(ctx context.Context, fields ...zapcore.Field) {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return
	}
	l.logFields = append(l.logFields, fields...)

}

func AddTag(ctx context.Context, key string, value interface{}) {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return
	}
	l.tags[key] = value
}

func GetTag(ctx context.Context, key string) interface{} {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return nil
	}
	return l.tags[key]
}

func GetRequestID(ctx context.Context) string {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return ""
	}
	return l.requestID
}

func GetElapsed(ctx context.Context) time.Duration {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return 0
	}
	return time.Since(l.startTime)
}

func GetDebugModeLogs(ctx context.Context) []string {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil || l.debugModeLogs == nil {
		return nil
	}
	return l.debugModeLogs.All()
}

func AddDebugModeLogs(ctx context.Context, logs ...string) {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil || l.debugModeLogs == nil {
		return
	}
	for _, log := range logs {
		l.debugModeLogs.Add(log)
	}
}

func ExistsLogger(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return false
	}
	if l.logger == nil {
		return false
	}
	return true
}

// Extract takes the call-scoped Logger from grpc_zap middleware.
func Logger(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		return log.AccessLogger()
	}

	//取出ctxStore
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		//todo 如何提醒调用方？

		return log.AccessLogger()
	}

	elapsed := time.Since(l.startTime).Milliseconds()
	fields := append(l.logFields, zap.Int64("elapsed", elapsed))
	fields = append(fields, zap.String("requestId", l.requestID))

	//如果不是debug模式，将日志写入访问日志文件
	if l.debugModeLogs == nil {
		return l.logger.Desugar().With(fields...).Sugar()
	} else { //否则，写入context的logs字段，最终返回给前端
		for _, field := range log.DefaultAccessLogFields() {
			fields = append(fields, field)
		}
		return log.NewDebugModeLogger(l.debugModeLogs).With(fields...).Sugar()
	}

}

func IsDebugModeOn(ctx context.Context) bool {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return false
	}
	return l.debugModeLogs != nil
}

//SetupContext 把访问日志的logger注入ctx中
//在本方法中生成了RequestID
//requestID暂时只在本进程内有效，不会传到grpc下游
//还生成了访问开始时间
func SetupContext(ctx context.Context) context.Context {
	id, _ := shortid.Generate()
	return SetupContextWithRequestID(ctx, id)
}

func CopyContext(ctx context.Context) context.Context {
	if ctx == nil {
		return SetupContext(nil)
	}

	//取出ctxStore
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		return SetupContext(nil)
	}

	newCtx := context.Background()
	return context.WithValue(newCtx, ctxStoreMarkerKey, l)
}

//服务链下游的服务可能已经有了requestID
func SetupContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	//生成本次访问相关的对象
	l := &ctxStore{
		logger:    log.AccessLogger(),
		startTime: time.Now(),
		requestID: requestID,
	}

	//注入
	if ginCtx, ok := ctx.(*gin.Context); ok { //gin.Context只支持string类型的key
		ginCtx.Set(ctxStoreMarkerKey, l)
		return ginCtx
	} else {
		return context.WithValue(ctx, ctxStoreMarkerKey, l)
	}
}

func TurnOnDebugMode(ctx context.Context) {
	l, ok := ctx.Value(ctxStoreMarkerKey).(*ctxStore)
	if !ok || l == nil {
		panic("cannot be here")
	}
	l.debugModeLogs = log.NewDebugModeLogs()
}
