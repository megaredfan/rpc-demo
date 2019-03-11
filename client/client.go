package client

import (
	"context"
	"errors"
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/transport"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrorShutdown = errors.New("client is shut down")

type RPCClient interface {
	Go(ctx context.Context, serviceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call
	Call(ctx context.Context, serviceMethod string, arg interface{}, reply interface{}) error
	Close() error
}

type Call struct {
	ServiceMethod string      // 服务名.方法名
	Args          interface{} // 参数
	Reply         interface{} // 返回值（指针类型）
	Error         error       // 错误信息
	Done          chan *Call  // 在调用结束时激活
}

type simpleClient struct {
	codec        codec.Codec
	rwc          io.ReadWriteCloser
	pendingCalls sync.Map
	mutex        sync.Mutex
	shutdown     bool
	option       Option
	seq          uint64
}

func NewRPCClient(network string, addr string, option Option) (RPCClient, error) {
	client := new(simpleClient)
	client.option = option

	client.codec = codec.GetCodec(option.SerializeType)

	tr := transport.NewTransport(option.TransportType)
	err := tr.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	client.rwc = tr

	go client.input()
	return client, nil
}

func (c *Call) done() {
	c.Done <- c
}

func (c *simpleClient) Go(ctx context.Context, serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply

	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		if cap(done) == 0 {
			log.Panic("rpc: done channel is unbuffered")
		}
	}
	call.Done = done

	c.send(ctx, call)

	return call
}

func (c *simpleClient) Call(ctx context.Context, serviceMethod string, args interface{}, reply interface{}) error {
	seq := atomic.AddUint64(&c.seq, 1)
	ctx = context.WithValue(ctx, protocol.RequestSeqKey, seq)

	canFn := func() {}
	if c.option.RequestTimeout != time.Duration(0) {
		ctx, canFn = context.WithTimeout(ctx, c.option.RequestTimeout)
		metaDataInterface := ctx.Value(protocol.MetaDataKey)
		var metaData map[string]string
		if metaDataInterface == nil {
			metaData = make(map[string]string)
		} else {
			metaData = metaDataInterface.(map[string]string)
		}
		metaData[protocol.RequestTimeoutKey] = c.option.RequestTimeout.String()
		ctx = context.WithValue(ctx, protocol.MetaDataKey, metaData)
	}

	done := make(chan *Call, 1)
	call := c.Go(ctx, serviceMethod, args, reply, done)
	select {
	case <-ctx.Done():
		canFn()
		c.pendingCalls.Delete(seq)
		call.Error = errors.New("client request time out")
	case <-call.Done:
	}
	return call.Error
}

func (c *simpleClient) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.shutdown = true

	c.pendingCalls.Range(func(key, value interface{}) bool {
		call, ok := value.(*Call)
		if ok {
			call.Error = ErrorShutdown
			call.done()
		}

		c.pendingCalls.Delete(key)
		return true
	})
	return nil
}

func (c *simpleClient) send(ctx context.Context, call *Call) {
	seq := ctx.Value(protocol.RequestSeqKey).(uint64)

	c.pendingCalls.Store(seq, call)

	request := protocol.NewMessage(c.option.ProtocolType)
	request.Seq = seq
	request.MessageType = protocol.MessageTypeRequest
	serviceMethod := strings.SplitN(call.ServiceMethod, ".", 2)
	request.ServiceName = serviceMethod[0]
	request.MethodName = serviceMethod[1]
	request.SerializeType = codec.MessagePack
	request.CompressType = protocol.CompressTypeNone
	if ctx.Value(protocol.MetaDataKey) != nil {
		request.MetaData = ctx.Value(protocol.MetaDataKey).(map[string]string)
	}

	requestData, err := c.codec.Encode(call.Args)
	if err != nil {
		log.Println(err)
		c.pendingCalls.Delete(seq)
		call.Error = err
		call.done()
		return
	}
	request.Data = requestData

	data := protocol.EncodeMessage(c.option.ProtocolType, request)

	_, err = c.rwc.Write(data)
	if err != nil {
		log.Println(err)
		c.pendingCalls.Delete(seq)
		call.Error = err
		call.done()
		return
	}
}

func (c *simpleClient) input() {
	var err error
	var response *protocol.Message
	for err == nil {
		response, err = protocol.DecodeMessage(c.option.ProtocolType, c.rwc)
		if err != nil {
			break
		}

		seq := response.Seq
		callInterface, _ := c.pendingCalls.Load(seq)
		call := callInterface.(*Call)
		c.pendingCalls.Delete(seq)

		switch {
		case call == nil:
			//请求已经被清理掉了，可能是已经超时了
		case response.Error != "":
			call.Error = errors.New(response.Error)
			call.done()
		default:
			err = c.codec.Decode(response.Data, call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
}
