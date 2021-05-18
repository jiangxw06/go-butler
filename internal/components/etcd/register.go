package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/jiangxw06/go-butler/internal/log"
	"google.golang.org/grpc/resolver"
	"time"
)

type (
	serviceRegister struct {
		serviceName  string
		registerHost string
		//timeout
		dialTimeout time.Duration
		//租约
		leaseID          clientv3.LeaseID
		keepAliveChannel <-chan *clientv3.LeaseKeepAliveResponse
	}
)

func RegisterService(serviceName string, port int) {
	//registerKey := GetEtcdKey(serviceName)
	registerHost := fmt.Sprintf("%v:%v", env.GetIP(), port)
	register := &serviceRegister{
		serviceName:  serviceName,
		registerHost: registerHost,
		dialTimeout:  time.Second,
	}
	go register.Start()
}

func (register *serviceRegister) Start() {
	err := register.doRegister()
	if err != nil {
		log.SysLogger().Error(fmt.Sprintf("register service failed! error is: %v", err))
		log.SysLogger().Sync()
		return
	}

	log.SysLogger().Infow("register grpc service success", "name", register.serviceName)

	//收到终止信号后unregister
	env.AddShutdownFunc(func() error {
		log.SysLogger().Infow("closing svcRegister", "name", register.serviceName)
		register.unregister()
		return nil
	})

	//keepAlive
	register.keepAlive()
}

func (register *serviceRegister) doRegister() error {
	//初始化租约
	ctx, _ := context.WithTimeout(context.Background(), register.dialTimeout)
	resp, err := GetEtcdClient().Grant(ctx, 10) //这个TTL足够长，使得之后调用keepAlive时不报错就行
	if err != nil {
		return fmt.Errorf("create lease error: %v", err)
	} else {
		//SysLogger().Trace().Str("serviceName",register.serviceName).Str("leaseID", fmt.Sprintf("%x", resp.ID)).
		//	Int64("ttl", int64(resp.TTL)).
		//	Msg("serviceRegister init lease")
	}
	register.leaseID = resp.ID

	//保持租约
	keepaliveChan, err := GetEtcdClient().KeepAlive(context.TODO(), resp.ID)
	if err != nil {
		return fmt.Errorf("keep lease alive error: %v", err)
	}
	register.keepAliveChannel = keepaliveChan

	//写入服务节点
	addr := resolver.Address{
		Addr: register.registerHost,
	}
	jsonStr, _ := json.Marshal(addr)
	ctx, _ = context.WithTimeout(context.Background(), register.dialTimeout)
	if _, err := GetEtcdClient().Put(ctx, register.getEtcdKey(), string(jsonStr), clientv3.WithLease(resp.ID)); err != nil {
		return err
	}

	return nil
}

func (register *serviceRegister) getEtcdKey() string {
	return GetEtcdKey(register.serviceName + "/" + register.registerHost)
}

func (register *serviceRegister) unregister() {
	for i := 0; i < 3; i++ {
		ctx, _ := context.WithTimeout(context.Background(), register.dialTimeout)
		resp, err := GetEtcdClient().Delete(ctx, register.getEtcdKey())
		if err != nil {
			log.SysLogger().Errorw("unregister service error", "serviceName", register.serviceName, "error", err)
		} else {
			log.SysLogger().Infow("unregister service success", "serviceName", register.serviceName, "deletedCount", resp.Deleted, "deletedKey", register.getEtcdKey())
			return
		}
		time.Sleep(time.Second)
	}
}

func (register *serviceRegister) revoke() error {
	ctx, cancel := context.WithTimeout(context.Background(), register.dialTimeout)
	_, err := GetEtcdClient().Revoke(ctx, register.leaseID)
	defer cancel()
	return err
}

func (register *serviceRegister) keepAlive() {
	for {
		if _, ok := <-register.keepAliveChannel; !ok {
			if err := register.revoke(); err != nil {
				log.SysLogger().Errorw("serviceRegister revoke error", "serviceName", register.serviceName, "error", err)
			}
			log.SysLogger().Infow("starting re-register service", "serviceName", register.serviceName)
			for {
				err := register.doRegister()
				if err != nil {
					log.SysLogger().Errorw("re-register service error", "serviceName", register.serviceName)
					time.Sleep(time.Second)
					continue
				} else {
					log.SysLogger().Infow("re-register service success", "serviceName", register.serviceName)
					break
				}
			}
		} else {
			//log.SysLogger().Info("keepAlive event received")
		}
	}
}
