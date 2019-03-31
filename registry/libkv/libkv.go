package libkv

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/share"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type KVRegistry struct {
	AppKey         string        //KVRegistry
	ServicePath    string        //数据存储的基本路径位置，比如/service/providers
	UpdateInterval time.Duration //定时拉取数据的时间间隔

	kv store.Store //store实例是一个封装过的客户端

	providersMu sync.RWMutex
	providers   []registry.Provider //本地缓存的列表

	watchersMu sync.Mutex
	watchers   []*Watcher //watcher列表
}

type Watcher struct {
	event chan *registry.Event
	exit  chan struct{}
}

func (w *Watcher) Next() (*registry.Event, error) {
	for {
		select {
		case r := <-w.event:
			return r, nil
		case <-w.exit:
			return nil, errors.New("watcher stopped")
		}
	}
}

func (w *Watcher) Close() {
	select {
	case <-w.exit:
		return
	default:
		close(w.exit)
	}
}

func NewKVRegistry(backend store.Backend,
	addrs []string,
	AppKey string,
	cfg *store.Config,
	ServicePath string,
	updateInterval time.Duration) registry.Registry {

	switch backend {
	case store.ZK:
		zookeeper.Register()
	case store.ETCD:
		etcd.Register()
	case store.CONSUL:
		consul.Register()
	case store.BOLTDB:
		boltdb.Register()
	}

	r := new(KVRegistry)
	r.AppKey = AppKey
	r.ServicePath = ServicePath
	r.UpdateInterval = updateInterval

	kv, err := libkv.NewStore(backend, addrs, cfg)
	if err != nil {
		log.Fatalf("cannot create kv registry: %v", err)
	}
	r.kv = kv

	basePath := r.ServicePath
	if basePath[0] == '/' { //路径不能以"/"开头
		basePath = basePath[1:]
		r.ServicePath = basePath
	}

	//先创建基本路径
	err = r.kv.Put(basePath, []byte("base path"), &store.WriteOptions{IsDir: true})
	if err != nil {
		log.Fatalf("cannot create regitry path %s: %v", r.ServicePath, err)
	}

	//显式拉取一次数据
	r.doGetServiceList()
	go func() {
		t := time.NewTicker(updateInterval)

		for range t.C {
			//定时拉取数据
			r.doGetServiceList()
		}
	}()

	go func() {
		//watch数据
		r.watch()
	}()
	return r
}

func (r *KVRegistry) watch() {
	//每次watch到数据后都需要重新watch，所以是一个死循环
	for {
		//监听appkey对应的目录,一旦父级目录的数据有变更就重新读取服务列表
		appkeyPath := constructServiceBasePath(r.ServicePath, r.AppKey)

		//监听时先检查路径是否存在
		if exist, _ := r.kv.Exists(appkeyPath); exist {
			lastUpdate := strconv.Itoa(int(time.Now().UnixNano()))
			err := r.kv.Put(appkeyPath, []byte(lastUpdate), &store.WriteOptions{IsDir: true})
			if err != nil {
				log.Printf("create path before watch error,  key %v", appkeyPath)
			}
		}
		ch, err := r.kv.Watch(appkeyPath, nil)
		if err != nil {
			log.Fatalf("error watch %v", err)
		}

		watchFinish := false
		for !watchFinish {
			//循环读取watch到的数据
			select {
			case pairs := <-ch:
				if pairs == nil {
					log.Printf("read finish")
					//watch数据结束，跳出这次循环
					watchFinish = true
				}

				//重新读取服务列表
				latestPairs, err := r.kv.List(appkeyPath)
				if err != nil {
					watchFinish = true
				}

				r.providersMu.RLock()
				list := r.providers
				r.providersMu.RUnlock()
				for _, p := range latestPairs {
					log.Printf("got provider %v", kv2Provider(p))
					list = append(list, kv2Provider(p))
				}

				r.providersMu.Lock()
				r.providers = list
				r.providersMu.Unlock()

				//通知watcher
				for _, w := range r.watchers {
					w.event <- &registry.Event{AppKey: r.AppKey, Providers: list}
				}
			}
		}
	}
}

func (r *KVRegistry) Register(option registry.RegisterOption, provider ...registry.Provider) {
	serviceBasePath := constructServiceBasePath(r.ServicePath, option.AppKey)

	for _, p := range provider {
		if p.Addr[0] == ':' {
			p.Addr = share.LocalIpV4() + p.Addr
		}
		key := serviceBasePath + p.Network + "@" + p.Addr
		data, _ := json.Marshal(p.Meta)
		err := r.kv.Put(key, data, nil)
		if err != nil {
			log.Printf("libkv register error: %v, provider: %v", err, p)
		}

		//注册时更新父级目录触发watch
		lastUpdate := strconv.Itoa(int(time.Now().UnixNano()))
		err = r.kv.Put(serviceBasePath, []byte(lastUpdate), nil)
		if err != nil {
			log.Printf("libkv register modify lastupdate error: %v, provider: %v", err, p)
		}
	}
}

func (r *KVRegistry) Unregister(option registry.RegisterOption, provider ...registry.Provider) {
	serviceBasePath := constructServiceBasePath(r.ServicePath, option.AppKey)

	for _, p := range provider {
		if p.Addr[0] == ':' {
			p.Addr = share.LocalIpV4() + p.Addr
		}
		key := serviceBasePath + p.Network + "@" + p.Addr
		err := r.kv.Delete(key)
		if err != nil {
			log.Printf("libkv unregister error: %v, provider: %v", err, p)
		}

		//注销时更新父级目录触发watch
		lastUpdate := strconv.Itoa(int(time.Now().UnixNano()))
		err = r.kv.Put(serviceBasePath, []byte(lastUpdate), nil)
		if err != nil {
			log.Printf("libkv register modify lastupdate error: %v, provider: %v", err, p)
		}
	}
}

func (r *KVRegistry) GetServiceList() []registry.Provider {
	r.providersMu.RLock()
	defer r.providersMu.RUnlock()
	return r.providers
}

func (r *KVRegistry) doGetServiceList() {
	path := constructServiceBasePath(r.ServicePath, r.AppKey)
	kvPairs, err := r.kv.List(path)

	var list []registry.Provider
	if err != nil {
		log.Printf("error get service list %v", err)
		return
	}

	for _, pair := range kvPairs {
		provider := kv2Provider(pair)
		list = append(list, provider)
	}
	log.Printf("get service list %v", list)
	r.providersMu.Lock()
	r.providers = list
	r.providersMu.Unlock()
}

func (r *KVRegistry) Watch() registry.Watcher {
	w := &Watcher{event: make(chan *registry.Event, 10), exit: make(chan struct{}, 10)}
	r.watchersMu.Lock()
	r.watchers = append(r.watchers, w)
	r.watchersMu.Unlock()
	return w
}

func (r *KVRegistry) Unwatch(watcher registry.Watcher) {
	var list []*Watcher
	r.watchersMu.Lock()
	defer r.watchersMu.Unlock()
	for _, w := range r.watchers {
		if w != watcher {
			list = append(list, w)
		}
	}
	r.watchers = list
}

func constructServiceBasePath(basePath string, appkey string) string {
	serviceBasePathBuffer := bytes.NewBufferString(basePath)
	serviceBasePathBuffer.WriteString("/")
	serviceBasePathBuffer.WriteString(appkey)
	serviceBasePathBuffer.WriteString("/")
	return serviceBasePathBuffer.String()
}

func kv2Provider(kv *store.KVPair) registry.Provider {
	provider := registry.Provider{}
	provider.ProviderKey = kv.Key
	networkAndAddr := strings.SplitN(kv.Key, "@", 2)
	provider.Network = networkAndAddr[0]
	provider.Addr = networkAndAddr[1]
	meta := make(map[string]interface{}, 0)
	json.Unmarshal(kv.Value, &meta)

	provider.Meta = meta
	return provider
}
