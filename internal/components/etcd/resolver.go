package etcd

import (
	"context"
	"encoding/json"
	"github.com/coreos/etcd/clientv3"
	"github.com/jiangxw06/go-butler/internal/log"
	"google.golang.org/grpc/resolver"
	"sync"
	"time"
)

const schema = "myschema"

type ResolverBuilder struct {
}

type myResolver struct {
	svcName     string
	registerKey string

	watchLock   sync.Mutex
	watchCancel context.CancelFunc
	watchChan   clientv3.WatchChan

	addrLock sync.Mutex
	cc       resolver.ClientConn
	key2Addr map[string]resolver.Address
}

var (
	MyResolverBuilder resolver.Builder = new(ResolverBuilder)
)

func InitResolver() {
	resolver.Register(MyResolverBuilder)
}

// Scheme return etcdv3 schema
func (r *ResolverBuilder) Scheme() string {
	return schema
}

// Build to resolver.Resolver
func (r *ResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	gw := &myResolver{
		svcName:     target.Endpoint,
		registerKey: GetEtcdKey(target.Endpoint), //参考GrpcClientConn方法的实现，这里Endpoint就是servicename
		cc:          cc,
		key2Addr:    make(map[string]resolver.Address),
	}
	gw.doResolveNow()
	return gw, nil
}

func (gw *myResolver) ResolveNow(options resolver.ResolveNowOptions) {
	log.SysLogger().Debugw("resolveNow", "svcName", gw.svcName)
	gw.doResolveNow()
}

func (gw *myResolver) Close() {
	gw.watchLock.Lock()
	defer gw.watchLock.Unlock()
	if gw.watchChan != nil {
		gw.watchCancel()
	}
	log.SysLogger().Infow("myResolver closed", "svcName", gw.svcName)
}

func getResolverAddress(nodeValue []byte) *resolver.Address {
	var addr resolver.Address
	err := json.Unmarshal(nodeValue, &addr)
	if err != nil {
		log.SysLogger().Warnw("grpc resolving: etcd value unmarshal failed", "nodeValue", string(nodeValue), "err", err)
		return nil
	}
	return &addr
}

func (gw *myResolver) watch() {
	for resp := range gw.watchChan {
		for _, e := range resp.Events {
			switch e.Type {
			case clientv3.EventTypePut:
				addr := getResolverAddress(e.Kv.Value)
				if addr != nil {
					gw.addAddr(string(e.Kv.Key), *addr)
					gw.notify()
				}
			case clientv3.EventTypeDelete:
				gw.removeAddr(string(e.Kv.Key))
				gw.notify()
			default:
				continue
			}
		}
	}
}

func (gw *myResolver) setAddrs(key2Addr map[string]resolver.Address) {
	gw.addrLock.Lock()
	defer gw.addrLock.Unlock()
	gw.key2Addr = make(map[string]resolver.Address)
	for k, addr := range key2Addr {
		gw.key2Addr[k] = addr
	}
	gw.notify()
}

func (gw *myResolver) notify() {
	addrList := make([]resolver.Address, 0, len(gw.key2Addr))
	var keys []string
	for k, v := range gw.key2Addr {
		addrList = append(addrList, v)
		keys = append(keys, k)
	}
	//log.SysLogger().Debugw("before gw.cc.UpdateState", "svcName", gw.svcName)
	gw.cc.UpdateState(resolver.State{
		Addresses: addrList,
	})
	log.SysLogger().Debugw("grpc resolver update", "svcName", gw.svcName, "keys", keys)
}

func (gw *myResolver) addAddr(key string, address resolver.Address) {
	gw.addrLock.Lock()
	defer gw.addrLock.Unlock()
	gw.key2Addr[key] = address
	gw.notify()
}

func (gw *myResolver) removeAddr(key string) {
	gw.addrLock.Lock()
	defer gw.addrLock.Unlock()
	delete(gw.key2Addr, key)
	gw.notify()
}

func (gw *myResolver) doResolveNow() {
	gw.watchLock.Lock()
	defer gw.watchLock.Unlock()
	//若已经建立了watcher，关闭wch以避免goroutine泄漏
	if gw.watchChan != nil {
		gw.watchCancel()
		gw.watchChan = nil
	}

	//请求etcd
	// Use serialized request so resolution still works if the target etcd
	// server is partitioned away from the quorum.
	getCtx, getCancel := context.WithTimeout(context.Background(), time.Second)
	defer getCancel()
	resp, err := GetEtcdClient().Get(getCtx, gw.registerKey+"/", clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		log.SysLogger().Errorw("resolveNow etcd get", "err", err, "svcName", gw.svcName)
		return
	}

	//更新cc状态
	m := make(map[string]resolver.Address)
	for _, kv := range resp.Kvs {
		addr := getResolverAddress(kv.Value)
		if addr != nil {
			m[string(kv.Key)] = *addr
		}
	}
	gw.setAddrs(m)

	//watch
	watchCtx, watchCancel := context.WithCancel(context.Background())
	opts := []clientv3.OpOption{clientv3.WithRev(resp.Header.Revision + 1), clientv3.WithPrefix(), clientv3.WithPrevKV()}
	gw.watchChan = GetEtcdClient().Watch(watchCtx, gw.registerKey+"/", opts...)
	gw.watchCancel = watchCancel
	go gw.watch()
}
