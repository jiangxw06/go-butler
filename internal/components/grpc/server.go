package grpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jiangxw06/go-butler/internal/components/etcd"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/jiangxw06/go-butler/util"
	"github.com/teris-io/shortid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"net"
	"time"
)

var kaep = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
	PermitWithoutStream: true,            // Allow pings even when there are no active streams
}

var kasp = keepalive.ServerParameters{
	MaxConnectionIdle:     15 * time.Second, // If a client is idle for 15 seconds, send a GOAWAY
	MaxConnectionAge:      30 * time.Second, // If any connection is alive for more than 30 seconds, send a GOAWAY
	MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
	Time:                  5 * time.Second,  // Ping the client if it is idle for 5 seconds to ensure the connection is still active
	Timeout:               1 * time.Second,  // Wait 1 second for the ping ack before assuming the connection is dead
}

//will block goroutine
func StartGrpcServer(serviceName string, server *grpc.Server) {
	svcOnce.Do(loadSvcConfig)
	config, ok := serviceConf.Register[serviceName]
	if !ok {
		log.SysLogger().Errorw("service not config", "service", serviceName)
		return
	}
	if config.Port == 0 {
		log.SysLogger().Errorw("service not config port or port = 0", "service", serviceName)
		return
	}

	//把Service注册到etcd中，并进行生命周期管理
	etcd.RegisterService(serviceName, config.Port)

	//收到终止信号时终止服务
	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("grpc server closing...", "service", serviceName)
		server.GracefulStop()
		log.SysLogger().Infow("grpc server closed", "service", serviceName)
		return nil
	})

	//serve and block
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", config.Port))
	if err != nil {
		log.SysLogger().Errorw("service net.Listen failed", "service", serviceName, "err", err)
		return
	}
	log.SysLogger().Infow("grpc service serving...", "name", serviceName)
	if err := server.Serve(listener); err != nil {
		log.SysLogger().Errorw("service error when serving", "error", err)
	} else {
		log.SysLogger().Infof("service {%v} stop serving", serviceName)
	}
}

//NewServer 支持自定义的Unary服务端拦截器
func NewServer(f func(server *grpc.Server), opt ...grpc.UnaryServerInterceptor) *grpc.Server {
	//拦截器
	var finalUnaryInts []grpc.UnaryServerInterceptor
	finalUnaryInts = append(finalUnaryInts, grpc_prometheus.UnaryServerInterceptor) //prometheus拦截器需要看到最后的ctx和err，所以放在第一个
	finalUnaryInts = append(finalUnaryInts, basicUnaryServerInterceptor)
	finalUnaryInts = append(finalUnaryInts, recoverUnaryServerInterceptor)
	finalUnaryInts = append(finalUnaryInts, timeoutUnaryServerInterceptor)
	finalUnaryInts = append(finalUnaryInts, opt...)

	unaryInterceptor := grpc_middleware.ChainUnaryServer(finalUnaryInts...)

	//暂时不支持扩展stream拦截器
	server := grpc.NewServer(
		grpc.UnaryInterceptor(unaryInterceptor),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
	)

	//调用方注册服务
	f(server)

	//支持服务反射，用于grpcui等
	reflection.Register(server)

	//指标收集
	grpc_prometheus.Register(server) //所有服务注册之后初始化各项指标
	grpc_prometheus.EnableHandlingTimeHistogram()

	return server
}

func basicUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	//context初始化
	requestID := getRequestIDFromIncomingCtx(ctx)
	debugModeOn := getDebugModeOnFromIncomingCtx(ctx)
	remoteIP := getRemoteIPFromIncommingCtx(ctx)
	ctx = contexts.SetupContextWithRequestID(ctx, requestID)
	if debugModeOn {
		contexts.TurnOnDebugMode(ctx)
	}
	contexts.AddLogFields(ctx, zap.String("entrance", info.FullMethod))
	contexts.AddLogFields(ctx, zap.String("entranceType", "grpc"))
	contexts.AddLogFields(ctx, zap.String("remoteIP", remoteIP))

	resp, err = handler(ctx, req)

	//debug模式返回日志
	if debugModeOn {
		logs := contexts.GetDebugModeLogs(ctx)
		for i := range logs {
			logs[i] = base64.StdEncoding.EncodeToString([]byte(logs[i]))
		}
		//str, _ := json.Marshal(logs)
		trailer := metadata.New(nil)
		trailer.Append("server-logs", logs...)
		//trailer := metadata.Pairs("server-logs", string(str))
		err := grpc.SetTrailer(ctx, trailer)
		if err != nil {
			contexts.Logger(ctx).Errorw("set grpc trailer err", "err", err)
		}
	}

	return resp, err
}

func getRemoteIPFromIncommingCtx(ctx context.Context) string {
	var addr string
	if pr, ok := peer.FromContext(ctx); ok {
		if tcpAddr, ok := pr.Addr.(*net.TCPAddr); ok {
			addr = tcpAddr.IP.String()
		} else {
			addr = pr.Addr.String()
		}
	}
	return addr
}

func getRequestIDFromIncomingCtx(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		id, _ := shortid.Generate()
		return id
	}
	header, ok := md["requestid"]
	if !ok || len(header) == 0 {
		id, _ := shortid.Generate()
		return id
	}
	id := header[0]
	if id == "" {
		id, _ := shortid.Generate()
		return id
	}
	return id
}

func getDebugModeOnFromIncomingCtx(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	header, ok := md["debug"]
	if !ok || len(header) == 0 {
		return false
	}
	return header[0] == "on"
}

//panic recover
func recoverUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err, ok := r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
			err = util.ErrorWithStack(err)
			contexts.Logger(ctx).Errorw("grpc panic recover", "err", err)
		}
	}()

	resp, err = handler(ctx, req)
	return resp, err
}

//timeout检测
func timeoutUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	//deadline, ok := ctx.Deadline()
	//if ok {
	//contexts.Logger(ctx).Debugw("grpc deadline set on clientside", "deadline", deadline)
	//}
	if ctx.Err() == context.Canceled {
		err = ctx.Err()
		contexts.Logger(ctx).Error("grpc already timeout(set on clientside) before handler")
		return
	}

	resp, err = handler(ctx, req)

	if err == nil && ctx.Err() == context.Canceled {
		err = ctx.Err()
		contexts.Logger(ctx).Error("grpc timeout(set on clientside) after handler")
	}

	return resp, err
}
