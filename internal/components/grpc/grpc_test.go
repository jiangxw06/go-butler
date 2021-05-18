package grpc

import (
	"context"
	"github.com/jiangxw06/go-butler/internal/components/grpc/testdata"
	"github.com/jiangxw06/go-butler/internal/contexts"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"google.golang.org/grpc"
	"testing"
	"time"
)

type (
	testServiceImpl struct {
		count int
	}
)

func (t *testServiceImpl) Test(ctx context.Context, empty *testdata.Empty) (*testdata.Count, error) {
	t.count++
	return &testdata.Count{Num: int32(t.count)}, nil
}

var _ testdata.TestServiceServer = new(testServiceImpl)

func startRegisterService(name string) {
	service := &testServiceImpl{}
	server := NewServer(func(server *grpc.Server) {
		testdata.RegisterTestServiceServer(server, service)
	})

	StartGrpcServer(name, server)
}

func configTest() {
	env.MergeConfig(`
[etcd]
    environment = "shawnjiang"
    addrs = ["10.18.8.240:4013", "10.18.8.241:4013", "10.18.8.242:4013"]

[service]
    #依赖的其他服务必须显式声明
    dependencies = ["test_service","test_service2","test_service3"]

    #本应用暴露的grpc服务，可以有多个，格式为service.register.{serviceName}
    [service.register.test_service]
        port = 9090
    [service.register.test_service2]
        port = 9091
    [service.register.test_service3]
        port = 9092
`)
}

func startDiscoverService(name string) {
	conn := MustGetGrpcClientConn(name)
	service := testdata.NewTestServiceClient(conn)
	go func() {
		for i := 0; i < 2<<30; i++ {
			time.Sleep(2 * time.Second)
			res, err := service.Test(contexts.SetupContext(nil), &testdata.Empty{})
			if err != nil {
				log.SysLogger().Error(err)
			}
			log.SysLogger().Info(res)
		}
	}()
}

func TestRegisterService(t *testing.T) {
	configTest()
	go startRegisterService("test_service")
	go startRegisterService("test_service2")
	go startRegisterService("test_service3")
	env.WaitClose()
}

func TestDiscoverService(t *testing.T) {
	configTest()
	startDiscoverService("test_service")
	env.WaitClose()
}

func TestDiscoverService2(t *testing.T) {
	configTest()
	startDiscoverService("test_service2")
	env.WaitClose()
}

func TestDiscoverService3(t *testing.T) {
	configTest()
	startDiscoverService("test_service3")
	env.WaitClose()
}
