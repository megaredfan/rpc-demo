package server

import (
	"encoding/json"
	"github.com/megaredfan/rpc-demo/registry"
	"log"
)

type SGServer interface {
	Register(rcvr interface{}, metaData map[string]string) error
	Serve(network string, addr string) error
	Close() error
}

type sgServer struct {
	SGOption  SGOption
	rpcServer RPCServer
}

func (s *sgServer) Register(rcvr interface{}, metaData map[string]string) error {
	return s.rpcServer.Register(rcvr, metaData)
}

func (s *sgServer) Close() error {
	return s.rpcServer.Close()
}

func (s *sgServer) Serve(network, addr string) error {
	serviceInfo, _ := json.Marshal(s.rpcServer.Services())
	provider := registry.Provider{
		ProviderKey: network + "@" + addr,
		Network:     network,
		Addr:        addr,
		Meta:        map[string]string{"services": string(serviceInfo)},
	}
	s.SGOption.Registry.Register(s.SGOption.RegisterOption, provider)
	log.Printf("registered provider %v for app %s", provider, s.SGOption.RegisterOption)
	return s.rpcServer.Serve(network, addr)
}
func (s *sgServer) AddShutDownHook(h ...ShutDownHook) {
	s.SGOption.ShutDownHooks = append(s.SGOption.ShutDownHooks, h...)
}
