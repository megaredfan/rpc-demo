package server

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/transport"
)

type ServeFunc func(network string, addr string) error
type ServeTransportFunc func(tr transport.Transport)
type HandleRequestFunc func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport)

type Wrapper interface {
	WrapServe(s *SGServer, serveFunc ServeFunc) ServeFunc
	WrapServeTransport(s *SGServer, transportFunc ServeTransportFunc) ServeTransportFunc
	WrapHandleRequest(s *SGServer, requestFunc HandleRequestFunc) HandleRequestFunc
}
