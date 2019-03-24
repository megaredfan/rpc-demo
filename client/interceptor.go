package client

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
	"log"
	"time"
)

type MetaDataWrapper struct {
}

func NewMetaDataWrapper() *MetaDataWrapper {
	return &MetaDataWrapper{}
}

func (w *MetaDataWrapper) WrapCall(option *SGOption, callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		ctx = wrapContext(ctx, option)
		return callFunc(ctx, ServiceMethod, arg, reply)
	}
}

func (w *MetaDataWrapper) WrapGo(option *SGOption, goFunc GoFunc) GoFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call {
		ctx = wrapContext(ctx, option)

		return goFunc(ctx, ServiceMethod, arg, reply, done)
	}
}

func wrapContext(ctx context.Context, option *SGOption) context.Context {
	timeout := time.Duration(0)
	deadline, ok := ctx.Deadline()
	if ok {
		//已经有了deadline了，此次请求覆盖原始的超时时间
		timeout = time.Until(deadline)
	}

	if timeout == time.Duration(0) && option.RequestTimeout != time.Duration(0) {
		//ctx里没有设置超时时间，用option中设置的超时时间
		timeout = option.RequestTimeout
	}

	ctx, _ = context.WithTimeout(ctx, timeout)

	metaDataInterface := ctx.Value(protocol.MetaDataKey)
	var metaData map[string]interface{}
	if metaDataInterface == nil {
		metaData = make(map[string]interface{})
	} else {
		metaData = metaDataInterface.(map[string]interface{})
	}
	metaData[protocol.RequestTimeoutKey] = uint64(timeout)

	if option.Auth != "" {
		metaData[protocol.AuthKey] = option.Auth
	}

	if auth, ok := ctx.Value(protocol.AuthKey).(string); ok {
		metaData[protocol.AuthKey] = auth
	}

	deadline, ok = ctx.Deadline()
	if ok {
		metaData[protocol.RequestDeadlineKey] = deadline.Unix()
	}
	ctx = context.WithValue(ctx, protocol.MetaDataKey, metaData)
	return ctx
}

type LogWrapper struct {
}

func NewLogWrapper() Wrapper {
	return &LogWrapper{}
}

func (*LogWrapper) WrapCall(option *SGOption, callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		log.Printf("before calling, ServiceMethod:%+v, arg:%+v", ServiceMethod, arg)
		err := callFunc(ctx, ServiceMethod, arg, reply)
		log.Printf("after calling, ServiceMethod:%+v, reply:%+v, error: %s", ServiceMethod, reply, err)
		return err
	}
}

func (*LogWrapper) WrapGo(option *SGOption, goFunc GoFunc) GoFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call {
		log.Printf("before going, ServiceMethod:%+v, arg:%+v", ServiceMethod, arg)
		return goFunc(ctx, ServiceMethod, arg, reply, done)
	}
}
