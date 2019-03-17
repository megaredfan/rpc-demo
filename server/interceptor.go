package server

import (
	"context"
	"encoding/json"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/transport"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

type DefaultServerWrapper struct {
}

func (w *DefaultServerWrapper) WrapServe(s *SGServer, serveFunc ServeFunc) ServeFunc {
	return func(network string, addr string) error {
		//注册shutdownHook
		go func(s *SGServer) {
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGTERM)
			sig := <-ch
			if sig.String() == "terminated" {
				for _, hook := range s.Option.ShutDownHooks {
					hook(s)
				}
				os.Exit(0)
			}
		}(s)

		serviceInfo, _ := json.Marshal(s.Services())
		provider := registry.Provider{
			ProviderKey: network + "@" + addr,
			Network:     network,
			Addr:        addr,
			Meta:        map[string]string{"services": string(serviceInfo)},
		}
		r := s.Option.Registry
		rOpt := s.Option.RegisterOption

		r.Register(rOpt, provider)
		log.Printf("registered provider %v for app %s", provider, rOpt)

		return serveFunc(network, addr)
	}
}

func (w *DefaultServerWrapper) WrapServeTransport(s *SGServer, transportFunc ServeTransportFunc) ServeTransportFunc {
	return transportFunc
}

func (w *DefaultServerWrapper) WrapHandleRequest(s *SGServer, requestFunc HandleRequestFunc) HandleRequestFunc {
	return func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport) {
		atomic.AddInt64(&s.requestInProcess, 1)
		requestFunc(ctx, request, response, tr)
		atomic.AddInt64(&s.requestInProcess, -1)
	}
}
