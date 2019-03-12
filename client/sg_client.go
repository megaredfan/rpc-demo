package client

import (
	"context"
	"github.com/megaredfan/rpc-demo/registery"
	"github.com/megaredfan/rpc-demo/selector"
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
	ServiceKey string
	FailMode   FailMode
	Retries    int
	Registry   registery.Registry
	Selector   selector.Selector

	*Option

	Meta map[string]string
}

type SGClient interface {
	Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) (*Call, error)
	Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error
}

type Handler interface {
	Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) context.Context
	Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) context.Context
}

type sgClient struct {
	shutdown bool
	option   *SGOption
	clients  map[string]RPCClient
	servers  map[string]registery.Provider
}

type CallOption func(op *SGOption)

func (c *sgClient) Go(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call, opts ...CallOption) (*Call, error) {
	if c.shutdown {
		return nil, ErrorShutdown
	}

	for _, op := range opts {
		op(c.option)
	}

	rpcClient, err := c.option.Selector.Next(ctx, ServiceMethod, arg)
	if err != nil {
		return nil, err
	}
	return rpcClient.Go(ctx, ServiceMethod, arg, reply, done), nil
}

func (c *sgClient) Call(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, opts ...CallOption) error {
	done := make(chan *Call, 1)
	call, err := c.Go(ctx, ServiceMethod, arg, reply, done)
	if err != nil && c.option.FailMode == FailFast {
		return err
	}
	call = <-call.Done
	return nil
}

func NewSGClient(option *SGOption) SGClient {
	s := &sgClient{option: option}
	s.servers = make(map[string]registery.Provider)
	providers := s.option.Registry.GetServiceList(option.ServiceKey)
	for _, p := range providers {
		s.servers[p.ProviderKey] = p
	}
	return s
}
