package client

import (
	"context"
	"errors"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/selector"
	"log"
	"sync"
	"time"
)

type SGClient interface {
	Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error
	Close() error
}

type sgClient struct {
	shutdown             bool
	option               SGOption
	clients              sync.Map //map[string]RPCClient
	clientsHeartbeatFail map[string]int
	breakers             sync.Map //map[string]CircuitBreaker
	watcher              registry.Watcher

	serversMu sync.RWMutex
	servers   []registry.Provider
	mu        sync.Mutex
}

func (c *sgClient) Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) (*Call, error) {
	if c.shutdown {
		return nil, ErrorShutdown
	}

	_, client, err := c.selectClient(ctx, ServiceMethod, arg)

	if err != nil {
		return nil, err
	}
	return c.wrapGo(client.Go)(ctx, ServiceMethod, arg, reply, done), nil

}

// ServiceError is an error from server.
type ServiceError string

func (e ServiceError) Error() string {
	return string(e)
}

func (c *sgClient) Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
	provider, rpcClient, err := c.selectClient(ctx, ServiceMethod, arg)

	if err != nil && c.option.FailMode == FailFast {
		return err
	}

	var connectErr error
	switch c.option.FailMode {
	case FailRetry:
		retries := c.option.Retries
		for retries > 0 {
			retries--

			if rpcClient != nil {
				err = c.wrapCall(rpcClient.Call)(ctx, ServiceMethod, arg, reply)
				if err == nil {
					if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
						breaker.(CircuitBreaker).Success()
					}
					return err
				}

				if err != nil {
					if _, ok := err.(ServiceError); ok {
						if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
							breaker.(CircuitBreaker).Fail(err)
						}
						return err
					}
				}
			}

			c.removeClient(provider.ProviderKey, rpcClient)
			rpcClient, connectErr = c.getClient(provider)
		}

		if err == nil {
			err = connectErr
		}

		if err == nil {
			if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
				breaker.(CircuitBreaker).Success()
			}
		} else {
			if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
				breaker.(CircuitBreaker).Fail(err)
			}
		}
		return err
	case FailOver:
		retries := c.option.Retries
		for retries > 0 {
			retries--

			if rpcClient != nil {
				err = c.wrapCall(rpcClient.Call)(ctx, ServiceMethod, arg, reply)
				if err == nil {
					if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
						breaker.(CircuitBreaker).Success()
					}
					return err
				}

				if err != nil {
					if _, ok := err.(ServiceError); ok {
						if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
							breaker.(CircuitBreaker).Fail(err)
						}
						return err
					}
				}
			}

			c.removeClient(provider.ProviderKey, rpcClient)
			provider, rpcClient, connectErr = c.selectClient(ctx, ServiceMethod, arg)
		}
		if err == nil {
			err = connectErr
		}
		if err == nil {
			if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
				breaker.(CircuitBreaker).Success()
			}
		} else {
			if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
				breaker.(CircuitBreaker).Fail(err)
			}
		}
		return err

	default: //FailFast or FailSafe
		err = c.wrapCall(rpcClient.Call)(ctx, ServiceMethod, arg, reply)
		if err != nil {
			if _, ok := err.(ServiceError); !ok {
				c.removeClient(provider.ProviderKey, rpcClient)
			}
		}

		if c.option.FailMode == FailSafe {
			err = nil
		}

		if err == nil {
			if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
				breaker.(CircuitBreaker).Success()
			}
		} else {
			if breaker, ok := c.breakers.Load(provider.ProviderKey); ok {
				breaker.(CircuitBreaker).Fail(err)
			}
		}

		return err
	}
}

func (c *sgClient) Close() error {
	c.shutdown = true

	c.mu.Lock()
	c.clients.Range(func(k, v interface{}) bool {
		if client, ok := v.(simpleClient); ok {
			c.removeClient(k.(string), &client)
		}
		return true
	})
	c.mu.Unlock()

	go func() {
		c.option.Registry.Unwatch(c.watcher)
		c.watcher.Close()
	}()

	return nil
}

func (c *sgClient) wrapCall(callFunc CallFunc) CallFunc {
	for _, wrapper := range c.option.Wrappers {
		callFunc = wrapper.WrapCall(&c.option, callFunc)
	}
	return callFunc
}

func (c *sgClient) wrapGo(goFunc GoFunc) GoFunc {
	for _, wrapper := range c.option.Wrappers {
		goFunc = wrapper.WrapGo(&c.option, goFunc)
	}
	return goFunc
}

func (c *sgClient) selectClient(ctx context.Context, ServiceMethod string, arg interface{}) (provider registry.Provider, client RPCClient, err error) {

	provider, err = c.option.Selector.Next(c.providers(), ctx, ServiceMethod, arg, c.option.SelectOption)
	if err != nil {
		return
	}

	client, err = c.getClient(provider)
	return
}

var ErrBreakerOpen = errors.New("breaker open")

func (c *sgClient) getClient(provider registry.Provider) (client RPCClient, err error) {
	key := provider.ProviderKey
	breaker, ok := c.breakers.Load(key)
	if ok && !breaker.(CircuitBreaker).AllowRequest() {
		return nil, ErrBreakerOpen
	}

	rc, ok := c.clients.Load(key)
	if ok {
		client = rc.(RPCClient)
		if !client.IsShutDown() {
			return
		} else {
			c.removeClient(key, client)
		}
	}

	rc, ok = c.clients.Load(key)
	if ok {
		client = rc.(RPCClient)
	} else {
		client, err = NewRPCClient(provider.Network, provider.Addr, c.option.Option)
		if err != nil {
			return
		}
		c.clients.Store(key, client)

		if c.option.CircuitBreakerThreshold > 0 && c.option.CircuitBreakerWindow > 0 {
			c.breakers.Store(key, NewDefaultCircuitBreaker(c.option.CircuitBreakerThreshold, c.option.CircuitBreakerWindow))
		}
	}
	return
}

func (c *sgClient) removeClient(clientKey string, client RPCClient) {
	c.clients.Delete(clientKey)
	if client != nil {
		client.Close()
	}
	c.breakers.Delete(clientKey)
}

func (c *sgClient) providers() []registry.Provider {
	c.serversMu.RLock()
	defer c.serversMu.RUnlock()
	return c.servers
}

func NewSGClient(option SGOption) SGClient {
	s := new(sgClient)
	s.option = option
	AddWrapper(&s.option, &MetaDataWrapper{}, &LogWrapper{}, &OpenTracingInterceptor{})

	providers := s.option.Registry.GetServiceList()
	s.watcher = s.option.Registry.Watch()
	go s.watchService(s.watcher)
	s.serversMu.Lock()
	defer s.serversMu.Unlock()
	for _, p := range providers {
		s.servers = append(s.servers, p)
	}
	if s.option.Heartbeat {
		go s.heartbeat()
		s.option.SelectOption.Filters = append(s.option.SelectOption.Filters,
			selector.DegradeProviderFilter())
	}

	if s.option.Tagged && s.option.Tags != nil {
		s.option.SelectOption.Filters = append(s.option.SelectOption.Filters,
			selector.TaggedProviderFilter(s.option.Tags))
	}

	return s
}

func (c *sgClient) watchService(watcher registry.Watcher) {
	if watcher == nil {
		return
	}
	for {
		event, err := watcher.Next()
		if err != nil {
			log.Println("watch service error:" + err.Error())
			break
		}

		c.serversMu.Lock()
		c.servers = event.Providers
		c.serversMu.Unlock()
	}
}

func (c *sgClient) heartbeat() {
	c.mu.Lock()
	if c.clientsHeartbeatFail == nil {
		c.clientsHeartbeatFail = make(map[string]int)
	}
	c.mu.Unlock()
	if c.option.HeartbeatInterval <= 0 {
		return
	}
	//根据指定的时间间隔发送心跳
	t := time.NewTicker(c.option.HeartbeatInterval)
	for range t.C {
		if c.shutdown {
			t.Stop()
			return
		}
		//遍历每个RPCClient进行心跳检查
		c.clients.Range(func(k, v interface{}) bool {
			err := v.(RPCClient).Call(context.Background(), "", "", nil)
			c.mu.Lock()
			if err != nil {
				//心跳失败进行计数
				if fail, ok := c.clientsHeartbeatFail[k.(string)]; ok {
					fail++
					c.clientsHeartbeatFail[k.(string)] = fail
				} else {
					c.clientsHeartbeatFail[k.(string)] = 1
				}
			} else {
				//心跳成功则进行恢复
				c.clientsHeartbeatFail[k.(string)] = 0
				c.serversMu.Lock()
				for i, p := range c.servers {
					if p.ProviderKey == k {
						delete(c.servers[i].Meta, protocol.ProviderDegradeKey)
					}
				}
				c.serversMu.Unlock()
			}
			c.mu.Unlock()
			//心跳失败次数超过阈值则进行降级
			if c.clientsHeartbeatFail[k.(string)] > c.option.HeartbeatDegradeThreshold {
				c.serversMu.Lock()
				for i, p := range c.servers {
					if p.ProviderKey == k {
						c.servers[i].Meta[protocol.ProviderDegradeKey] = true
					}
				}
				c.serversMu.Unlock()
			}
			return true
		})
	}
}
