package client

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
	"log"
	"time"
)

type MetaDataWrapper struct {
	Option *SGOption
}

func NewMetaDataWrapper(s *sgClient) *MetaDataWrapper {
	return &MetaDataWrapper{Option: &s.option}
}

func (w *MetaDataWrapper) WrapCall(callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		ctx = wrapContext(ctx, w.Option)
		return callFunc(ctx, ServiceMethod, arg, reply)
	}
}

func (w *MetaDataWrapper) WrapGo(goFunc GoFunc) GoFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call {
		ctx = wrapContext(ctx, w.Option)

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

	deadline, ok = ctx.Deadline()
	if ok {
		metaData[protocol.RequestDeadlineKey] = deadline.Unix()
	}
	ctx = context.WithValue(ctx, protocol.MetaDataKey, metaData)
	return ctx
}

type LogWrapper struct {
}

func (*LogWrapper) WrapCall(callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		log.Println("before calling")
		err := callFunc(ctx, ServiceMethod, arg, reply)
		log.Println("after...")
		return err
	}
}

func (*LogWrapper) WrapGo(goFunc GoFunc) GoFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call {
		log.Println("before calling")
		return goFunc(ctx, ServiceMethod, arg, reply, done)
	}
}
