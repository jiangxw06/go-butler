package grpc

import (
	"context"
	"encoding/base64"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jiangxw06/go-butler/internal/components/etcd"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"sync"
	"time"
)

var (
	rsvOnce sync.Once
	kacp    = keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}
)

func MustGetGrpcClientConn(serviceName string, unaryInterceptors ...grpc.UnaryClientInterceptor) *grpc.ClientConn {
	conn, err := GetGrpcClientConn(serviceName, unaryInterceptors...)
	if err != nil {
		log.SysLogger().Error(err)
		panic(err)
	}
	return conn
}

//暂时不支持Stream客户端拦截器
func GetGrpcClientConn(serviceName string, unaryInterceptors ...grpc.UnaryClientInterceptor) (*grpc.ClientConn, error) {
	rsvOnce.Do(etcd.InitResolver)
	svcOnce.Do(loadSvcConfig)
	//必须在配置文件中声明依赖
	err := validateDependency(serviceName)
	if err != nil {
		log.SysLogger().Error(err)
		return nil, err
	}

	target := etcd.MyResolverBuilder.Scheme() + "://authority/" + serviceName

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target,
		grpc.WithInsecure(),
		grpc.WithBalancerName(roundrobin.Name),
		grpc.WithBlock(),
		grpcUnaryClientInterceptorOption(unaryInterceptors...),
		grpc.WithKeepaliveParams(kacp),
	)

	if err != nil {
		err = errors.Errorf("Connect etcd error in {%v} GetGrpcClientConn! : %v", serviceName, err)
		log.SysLogger().Error(err)
		return nil, err
	}
	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("grpcConn closing...", "serviceName", serviceName)
		err := conn.Close()
		if err != nil {
			log.SysLogger().Errorw("close grpcConn error", "serviceName", serviceName, "error", err)
		} else {
			log.SysLogger().Infow("grpcConn closed", "serviceName", serviceName)
		}
		return nil
	})
	log.SysLogger().Infof("%v Connect etcd success in GetGrpcClientConn!", serviceName)
	return conn, nil
}

func grpcUnaryClientInterceptorOption(opt ...grpc.UnaryClientInterceptor) grpc.DialOption {
	var finalInts []grpc.UnaryClientInterceptor
	finalInts = append(finalInts, grpc_prometheus.UnaryClientInterceptor)
	finalInts = append(finalInts, basicUnaryClientInterceptor)
	finalInts = append(finalInts, opt...)
	unaryInt := grpc_middleware.ChainUnaryClient(finalInts...)
	return grpc.WithUnaryInterceptor(unaryInt)
}

func validateDependency(serviceName string) error {
	declared := false
	for _, v := range serviceConf.Dependencies {
		if v == serviceName {
			declared = true
		}
	}
	if !declared {
		return errors.Errorf("service dependency name = {%v} not declared", serviceName)
	}
	return nil
}

func basicUnaryClientInterceptor(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	//requestid,debug和context
	requestID := contexts.GetRequestID(ctx)
	debugModeOn := contexts.IsDebugModeOn(ctx)
	m := map[string]string{}
	if requestID != "" {
		m["requestid"] = requestID
	}
	if debugModeOn {
		m["debug"] = "on"
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.New(m))

	//超时
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	startTime := time.Now()

	//执行
	var trailer metadata.MD
	opts = append(opts, grpc.Trailer(&trailer))
	err := invoker(ctx, method, req, reply, cc, opts...)

	//日志
	l := contexts.Logger(ctx).With(
		"method", method,
		"duration", time.Since(startTime).Milliseconds())
	if err != nil {
		l.Errorw("grpc outgoing error",
			"err", err,
			"cc", cc,
			"cc.State", cc.GetState(),
			"cc.MethodConfig", cc.GetMethodConfig(method),
			"cc.Target", cc.Target())
	} else {
		l.Debug("grpc outgoing")
	}

	if debugModeOn {
		serverLogs, ok := trailer["server-logs"]
		if ok {
			for _, l := range serverLogs {
				bytes, err := base64.StdEncoding.DecodeString(l)
				if err == nil {
					contexts.AddDebugModeLogs(ctx, string(bytes))
				}
			}
			//var logs []string
			//err := json.Unmarshal([]byte(serverLogs[0]),&logs)
			//for i, log := range logs {
			//	logs[i] = fmt.Sprintf(log + "\n")
			//}

		}
	}

	return err
}
