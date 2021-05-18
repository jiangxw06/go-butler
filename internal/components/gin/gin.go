package gin

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	_ "github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/prometheus"
	"github.com/jiangxw06/go-butler/util"
	"go.uber.org/zap"
	"time"
)

func NewEngine() *gin.Engine {
	engine := gin.New()
	engine.RemoveExtraSlash = true
	pprof.Register(engine)
	engine.Use(basicHandlerFunc)
	engine.Use(LoggerMiddleWare)
	if env.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	return engine
}

//func pathHandlerFunc(ctx *gin.Context) {
//	url := ctx.Request.URL
//	url.Path = strings.Replace(url.Path, "//","/",-1)
//
//	//其他流程
//	ctx.Next()
//}

func basicHandlerFunc(ctx *gin.Context) {
	//初始化context
	contexts.SetupContext(ctx)
	//debug模式
	mode := ctx.Query("debug")
	if mode == "on" {
		contexts.TurnOnDebugMode(ctx)
	}
	//增加url到日志中
	contexts.AddLogFields(ctx, zap.String("entrance", ctx.Request.URL.Path))
	contexts.AddLogFields(ctx, zap.String("entranceType", "http"))

	//cors跨域
	config := cors.Config{
		AllowCredentials: true,
		AllowWebSockets:  true,
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTION"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Accept-Encoding", "X-CSRF-Token", "Authorization", "Cache-Control", "X-Requested-With", "Token"},
		MaxAge:           1 * time.Minute,
	}
	cors.New(config)

	//panic recover
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
			err = util.ErrorWithStack(err)
			contexts.Logger(ctx).Errorw("http panic recover", "err", err)
		}
	}()

	//其他流程
	ctx.Next()

	//写response
	body := assembleBody(ctx)
	renderResp(ctx, body)

	//http请求次数及耗时打点
	elapsedSecs := contexts.GetElapsed(ctx).Seconds()
	prometheus.HttpReqDuration("self", ctx.Request.URL.Path, ctx.Writer.Status()).Observe(elapsedSecs)
}

func TimeoutMiddleWare() gin.HandlerFunc {
	panic("not implemented")
}
