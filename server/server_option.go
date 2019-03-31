package server

import (
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/transport"
	"time"
)

type Option struct {
	AppKey         string
	Registry       registry.Registry
	RegisterOption registry.RegisterOption
	Wrappers       []Wrapper
	ShutDownWait   time.Duration
	ShutDownHooks  []ShutDownHook
	Tags           map[string]string

	ProtocolType  protocol.ProtocolType
	SerializeType codec.SerializeType
	CompressType  protocol.CompressType
	TransportType transport.TransportType
}

var DefaultOption = Option{
	ShutDownWait:  time.Second * 12,
	ProtocolType:  protocol.Default,
	SerializeType: codec.MessagePack,
	CompressType:  protocol.CompressTypeNone,
	TransportType: transport.TCPTransport,
}

type ShutDownHook func(s *SGServer)
