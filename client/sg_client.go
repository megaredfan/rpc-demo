package client

import (
	"context"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/selector"
	"log"
	"sync"
)

type FailMode byte

const (
	FailFast FailMode = iota
	FailOver
	FailSafe
	FailRetry
	FailBack
	Broadcast
	Fork
)

type SGOption struct {
	ServiceKey    string
	FailMode      FailMode
	Retries       int
	Registry      registry.Registry
	Selector      selector.Selector
	SelectOptions []selector.SelectOption
	CallWrappers  []CallFuncWrapper

	Option

	Meta map[string]string
}

var DefaultSGOption = SGOption{
	ServiceKey: "",
	FailMode:   FailFast,
	Retries:    0,
	Selector:   selector.NewRandomSelector(),

	Option: DefaultOption,

	Meta: make(map[string]string),
}

type SGClient interface {
	Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error
}

type Invoker interface {
	Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error
}

type sgClient struct {
	shutdown  bool
	option    SGOption
	clients   sync.Map //map[string]RPCClient
	serversMu sync.RWMutex
	servers   []registry.Provider
}

type CallOption func(op *SGOption)

func (c *sgClient) Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) (*Call, error) {
	if c.shutdown {
		return nil, ErrorShutdown
	}

	_, client, err := c.selectClient(ctx, ServiceMethod, arg)

	if err != nil {
		return nil, err
	}
	return client.Go(ctx, ServiceMethod, arg, reply, done), nil

}

// ServiceError is an error from server.
type ServiceError string

func (e ServiceError) Error() string {
	return string(e)
}

func (c *sgClient) Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
	provider, rpcClient, err := c.selectClient(ctx, ServiceMethod, arg, c.option.SelectOptions...)

	if err != nil && c.option.FailMode == FailFast {
		return err
	}

	switch c.option.FailMode {
	case FailRetry:
		retries := c.option.Retries
		for retries > 0 {
			retries--

			if rpcClient != nil {
				err = c.wrapCall(rpcClient.Call)(ctx, ServiceMethod, arg, reply)
				//err = rpcClient.Call(ctx, ServiceMethod, arg, reply)
				if err == nil {
					return err
				}

				if err != nil {
					if _, ok := err.(ServiceError); ok {
						return err
					}
				}
			}

			c.removeClient(provider.ProviderKey, rpcClient)
			rpcClient, err = c.getClient(provider)
		}

		return err
	case FailOver:
		retries := c.option.Retries
		for retries > 0 {
			retries--

			if rpcClient != nil {
				err = c.wrapCall(rpcClient.Call)(ctx, ServiceMethod, arg, reply)
				//err = rpcClient.Call(ctx, ServiceMethod, arg, reply)
				if err == nil {
					return err
				}

				if err != nil {
					if _, ok := err.(ServiceError); ok {
						return err
					}
				}
			}

			c.removeClient(provider.ProviderKey, rpcClient)
			provider, rpcClient, err = c.selectClient(ctx, ServiceMethod, arg)
		}

		return err
	default: //FailFast
		err = c.wrapCall(rpcClient.Call)(ctx, ServiceMethod, arg, reply)
		//err = rpcClient.Call(ctx, ServiceMethod, arg, reply)
		if err != nil {
			if _, ok := err.(ServiceError); !ok {
				c.removeClient(provider.ProviderKey, rpcClient)
			}
		}

		return err
	}
}

func (c *sgClient) wrapCall(callFunc CallFunc) CallFunc {
	for _, wrapper := range c.option.CallWrappers {
		callFunc = wrapper(callFunc)
	}
	return callFunc
}

type CallFunc func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error

type CallFuncWrapper func(callFunc CallFunc) CallFunc

func (c *sgClient) selectClient(ctx context.Context, ServiceMethod string, arg interface{}, opts ...selector.SelectOption) (provider registry.Provider, client RPCClient, err error) {

	provider, err = c.option.Selector.Next(c.providers(), ctx, ServiceMethod, arg, opts...)
	if err != nil {
		return
	}
	clientKey := provider.ProviderKey

	rc, ok := c.clients.Load(clientKey)
	if ok {
		client := rc.(RPCClient)
		if client.IsShutDown() {
			c.clients.Delete(clientKey)
		}
	}

	rc, ok = c.clients.Load(clientKey)
	if ok {
		client = rc.(RPCClient)
	} else {
		client, err = NewRPCClient(provider.Network, provider.Addr, c.option.Option)
		if err != nil {
			return
		}
		c.clients.Store(clientKey, client)
	}

	return
}

func (c *sgClient) getClient(provider registry.Provider) (client RPCClient, err error) {
	key := provider.ProviderKey
	rc, ok := c.clients.Load(key)
	if ok {
		client = rc.(RPCClient)
		if !client.IsShutDown() {
			return
		} else {
			c.clients.Delete(key)
			client.Close()
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
	}
	return
}

func (c *sgClient) removeClient(clientKey string, client RPCClient) {
	c.clients.Delete(clientKey)
	if client != nil {
		client.Close()
	}
}

func (c *sgClient) providers() []registry.Provider {
	c.serversMu.RLock()
	defer c.serversMu.RUnlock()
	return c.servers
}

func NewSGClient(option SGOption) SGClient {
	s := &sgClient{option: option}
	providers := s.option.Registry.GetServiceList(option.ServiceKey)
	watcher := s.option.Registry.Watch(option.ServiceKey)
	log.Printf("watcher: %v", watcher)
	go func() {
		for {
			event, err := watcher.Next()
			log.Printf("received watch event:%v", event)
			if err != nil {
				log.Println(err)
				break
			}

			if event.ServiceKey == s.option.ServiceKey {
				switch event.Action {
				case registry.Create:
					s.serversMu.Lock()
					for _, p := range providers {
						if p.ProviderKey != event.Provider.ProviderKey {
							s.servers = append(s.servers, p)
						}
					}
					s.servers = append(s.servers, event.Provider)
					s.serversMu.Unlock()
				case registry.Update:
					s.serversMu.Lock()
					for _, p := range providers {
						if p.ProviderKey != event.Provider.ProviderKey {
							s.servers = append(s.servers, p)
						}
					}
					s.servers = append(s.servers, event.Provider)
					s.serversMu.Unlock()
				case registry.Delete:
					s.serversMu.Lock()
					for _, p := range providers {
						if p.ProviderKey != event.Provider.ProviderKey {
							s.servers = append(s.servers, p)
						}
					}
					s.serversMu.Unlock()
				}
			}

		}
	}()
	s.serversMu.Lock()
	defer s.serversMu.Unlock()
	for _, p := range providers {
		s.servers = append(s.servers, p)
	}
	return s
}
