package server

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/share/metadata"
	"github.com/megaredfan/rpc-demo/transport"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

type DefaultServerWrapper struct {
	defaultServerInterceptor
}

func (w *DefaultServerWrapper) WrapServe(s *SGServer, serveFunc ServeFunc) ServeFunc {
	return func(network string, addr string, meta map[string]interface{}) error {
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

		if meta == nil {
			meta = make(map[string]interface{})
		}
		if len(s.Option.Tags) > 0 {
			meta["tags"] = s.Option.Tags
		}
		meta["services"] = s.Services()
		provider := registry.Provider{
			ProviderKey: network + "@" + addr,
			Network:     network,
			Addr:        addr,
			Meta:        meta,
		}
		r := s.Option.Registry
		rOpt := s.Option.RegisterOption

		r.Register(rOpt, provider)
		log.Printf("registered provider %v for app %s", provider, rOpt)

		return serveFunc(network, addr, meta)
	}
}

func (w *DefaultServerWrapper) WrapHandleRequest(s *SGServer, requestFunc HandleRequestFunc) HandleRequestFunc {
	return func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport) {
		ctx = metadata.WithMeta(ctx, request.MetaData)
		atomic.AddInt64(&s.requestInProcess, 1)
		requestFunc(ctx, request, response, tr)
		atomic.AddInt64(&s.requestInProcess, -1)
	}
}

func (w *DefaultServerWrapper) WrapClose(s *SGServer, closeFunc CloseFunc) CloseFunc {
	return func() error {
		provider := registry.Provider{
			ProviderKey: s.network + "@" + s.addr,
			Network:     s.network,
			Addr:        s.addr,
		}
		r := s.Option.Registry
		rOpt := s.Option.RegisterOption

		r.Unregister(rOpt, provider)
		log.Printf("unregistered provider %v for app %s", provider, rOpt)

		return closeFunc()
	}
}
