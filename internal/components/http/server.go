package http

import (
	"context"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/pkg/errors"
	"net/http"
	_ "net/http/pprof"
	"time"
)

type (
	Server struct {
		HttpServer *http.Server
		Name       string
	}
)

func GetServer(name string) (*Server, error) {
	httpOnce.Do(initHttpConfig)
	httpServer, ok := httpServerMap[name]
	if !ok {
		return nil, errors.Errorf("http server {%v} not config", name)
	}
	return httpServer, nil
}

func MustGetServer(name string) *Server {
	s, err := GetServer(name)
	if err != nil {
		panic(err)
	}
	return s
}

func StartHttpServer(server *Server) {
	log.SysLogger().Infow("httpServer started", "name", server.Name)
	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("httpServer closing...", "name", server.Name)
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		if err := server.HttpServer.Shutdown(ctx); err != nil {
			log.SysLogger().Errorw("shutdown httpServer error", "name", server.Name, "error", err)
		} else {
			log.SysLogger().Infow("shutdown httpServer success", "name", server.Name)
		}
		return nil
	})
	// 开启http监听
	if err := server.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.SysLogger().Errorw("start http server error", "name", server.Name, "error", err)
		return
	}
}
