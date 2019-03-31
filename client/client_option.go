package client

import (
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/selector"
	"github.com/megaredfan/rpc-demo/transport"
	"math"
	"time"
)

type Option struct {
	ProtocolType  protocol.ProtocolType
	SerializeType codec.SerializeType
	CompressType  protocol.CompressType
	TransportType transport.TransportType

	DialTimeout    time.Duration
	RequestTimeout time.Duration

	Heartbeat                 bool
	HeartbeatInterval         time.Duration
	HeartbeatDegradeThreshold int

	Tagged bool
	Tags   map[string]string
}

var DefaultOption = Option{
	ProtocolType:  protocol.Default,
	SerializeType: codec.MessagePack,
	CompressType:  protocol.CompressTypeNone,
	TransportType: transport.TCPTransport,

	Heartbeat:                 false,
	HeartbeatInterval:         0,
	HeartbeatDegradeThreshold: math.MaxInt32,
	Tagged:                    false,
}

type FailMode byte

const (
	FailFast FailMode = iota
	FailOver
	FailRetry
	FailSafe
)

type SGOption struct {
	AppKey       string
	RemoteAppkey string
	FailMode     FailMode
	Retries      int
	Registry     registry.Registry
	Selector     selector.Selector
	SelectOption selector.SelectOption
	Wrappers     []Wrapper

	Option
	Auth                    string
	CircuitBreakerThreshold uint64
	CircuitBreakerWindow    time.Duration
	Meta                    map[string]string
}

func AddWrapper(o *SGOption, w ...Wrapper) *SGOption {
	o.Wrappers = append(o.Wrappers, w...)
	return o
}

var DefaultSGOption = SGOption{
	AppKey:   "",
	FailMode: FailFast,
	Retries:  0,
	Selector: selector.NewRandomSelector(),

	Option: DefaultOption,

	Meta: make(map[string]string),
}
