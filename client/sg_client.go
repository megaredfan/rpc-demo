package client

import (
	"context"
	"github.com/megaredfan/rpc-demo/registry"
	"log"
	"sync"
)

type SGClient interface {
	Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error
}

type sgClient struct {
	shutdown  bool
	option    SGOption
	clients   sync.Map //map[string]RPCClient
	serversMu sync.RWMutex
	servers   []registry.Provider
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
			rpcClient, connectErr = c.getClient(provider)
		}

		if err == nil {
			err = connectErr
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
			provider, rpcClient, connectErr = c.selectClient(ctx, ServiceMethod, arg)
		}
		if err == nil {
			err = connectErr
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
		return err
	}
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
	s := new(sgClient)
	s.option = option
	AddWrapper(&s.option, NewMetaDataWrapper(), NewLogWrapper())

	providers := s.option.Registry.GetServiceList()
	watcher := s.option.Registry.Watch()
	go s.watchService(watcher)
	s.serversMu.Lock()
	defer s.serversMu.Unlock()
	for _, p := range providers {
		s.servers = append(s.servers, p)
	}
	return s
}

func (c *sgClient) watchService(watcher registry.Watcher) {
	if watcher == nil {
		return
	}
	for {
		event, err := watcher.Next()
		//log.Printf("received watch event:%v", event)
		if err != nil {
			log.Println("watch service error:" + err.Error())
			break
		}

		if event.AppKey == c.option.AppKey {
			switch event.Action {
			case registry.Create:
				c.serversMu.Lock()
				for _, ep := range event.Providers {
					exists := false
					for _, p := range c.servers {
						if p.ProviderKey == ep.ProviderKey {
							exists = true
						}
					}
					if !exists {
						c.servers = append(c.servers, ep)
					}
				}

				c.serversMu.Unlock()
			case registry.Update:
				c.serversMu.Lock()
				for _, ep := range event.Providers {
					for i := range c.servers {
						if c.servers[i].ProviderKey == ep.ProviderKey {
							c.servers[i] = ep
						}
					}
				}
				c.serversMu.Unlock()
			case registry.Delete:
				c.serversMu.Lock()
				var newList []registry.Provider
				for _, p := range c.servers {
					for _, ep := range event.Providers {
						if p.ProviderKey != ep.ProviderKey {
							newList = append(newList, p)
						}
					}
				}
				c.servers = newList
				c.serversMu.Unlock()
			}
		}

	}
}
