package client

import "context"

type CallFunc func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error
type GoFunc func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call

type Wrapper interface {
	WrapCall(option *SGOption, callFunc CallFunc) CallFunc
	WrapGo(option *SGOption, goFunc GoFunc) GoFunc
}

type defaultClientInterceptor struct {
}

func (defaultClientInterceptor) WrapCall(option *SGOption, callFunc CallFunc) CallFunc {
	return callFunc
}

func (defaultClientInterceptor) WrapGo(option *SGOption, goFunc GoFunc) GoFunc {
	return goFunc
}
