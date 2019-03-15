package server

import (
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/transport"
)

type Option struct {
	ProtocolType  protocol.ProtocolType
	SerializeType codec.SerializeType
	CompressType  protocol.CompressType
	TransportType transport.TransportType
}

var DefaultOption = Option{
	ProtocolType:  protocol.Default,
	SerializeType: codec.MessagePack,
	CompressType:  protocol.CompressTypeNone,
	TransportType: transport.TCPTransport,
}

type Wrapper interface {
	WrapAccept()
	WrapServe()
	WrapServeConn()
}

type ShutDownHook func(s *sgServer)

type SGOption struct {
	AppKey         string
	Registry       registry.Registry
	RegisterOption registry.RegisterOption
	Wrappers       []Wrapper
	ShutDownHooks  []ShutDownHook
	Option
}

var DefaultSGOption = SGOption{
	Option: DefaultOption,
}

func NewSGServer(opt SGOption) SGServer {
	s := new(sgServer)
	s.SGOption = opt
	s.rpcServer = NewRPCServer(opt.Option)
	return s
}
