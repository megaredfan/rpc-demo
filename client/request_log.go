package client

import (
	"context"
	"log"
)

type LogWrapper struct {
	defaultClientInterceptor
}

func (*LogWrapper) WrapCall(option *SGOption, callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		log.Printf("before calling, ServiceMethod:%+v, arg:%+v", ServiceMethod, arg)
		err := callFunc(ctx, ServiceMethod, arg, reply)
		log.Printf("after calling, ServiceMethod:%+v, reply:%+v, error: %v", ServiceMethod, reply, err)
		return err
	}
}

func (*LogWrapper) WrapGo(option *SGOption, goFunc GoFunc) GoFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call {
		log.Printf("before going, ServiceMethod:%+v, arg:%+v", ServiceMethod, arg)
		return goFunc(ctx, ServiceMethod, arg, reply, done)
	}
}
