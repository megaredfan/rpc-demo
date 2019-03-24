package server

import (
	"context"
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

		provider := registry.Provider{
			ProviderKey: network + "@" + addr,
			Network:     network,
			Addr:        addr,
			Meta:        map[string]interface{}{"services": s.Services()},
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

type ServerAuthInterceptor struct {
	authFunc AuthFunc
}

func NewAuthInterceptor(authFunc AuthFunc) Wrapper {
	return &ServerAuthInterceptor{authFunc}
}

func (*ServerAuthInterceptor) WrapServe(s *SGServer, serveFunc ServeFunc) ServeFunc {
	return serveFunc
}

func (*ServerAuthInterceptor) WrapServeTransport(s *SGServer, transportFunc ServeTransportFunc) ServeTransportFunc {
	return transportFunc
}

func (sai *ServerAuthInterceptor) WrapHandleRequest(s *SGServer, requestFunc HandleRequestFunc) HandleRequestFunc {
	return func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport) {
		if auth, ok := ctx.Value(protocol.AuthKey).(string); ok {
			//鉴权通过则执行业务逻辑
			if sai.authFunc(auth) {
				requestFunc(ctx, response, response, tr)
				return
			}
		}
		//鉴权失败则返回异常
		s.writeErrorResponse(response, tr, "auth failed")
	}
}

func (*ServerAuthInterceptor) WrapClose(s *SGServer, closeFunc CloseFunc) CloseFunc {
	return closeFunc
}
