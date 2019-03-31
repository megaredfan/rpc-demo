package client

import (
	"context"
	"errors"
	"github.com/megaredfan/rpc-demo/share/ratelimit"
)

type RateLimitInterceptor struct {
	defaultClientInterceptor
	Limit ratelimit.RateLimiter
}

var ErrRateLimited = errors.New("request limited")

func (r *RateLimitInterceptor) WrapCall(option *SGOption, callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		if r.Limit != nil {
			if r.Limit.TryAcquire() {
				return callFunc(ctx, ServiceMethod, arg, reply)
			} else {
				return ErrRateLimited
			}
		} else {
			return callFunc(ctx, ServiceMethod, arg, reply)
		}
	}
}

func (r *RateLimitInterceptor) WrapGo(option *SGOption, goFunc GoFunc) GoFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}, done chan *Call) *Call {
		if r.Limit != nil {
			if r.Limit.TryAcquire() {
				return goFunc(ctx, ServiceMethod, arg, reply, done)
			} else {
				call := &Call{
					ServiceMethod: ServiceMethod,
					Args:          arg,
					Reply:         nil,
					Error:         ErrRateLimited,
					Done:          done,
				}
				done <- call
				return call
			}
		} else {
			return goFunc(ctx, ServiceMethod, arg, reply, done)
		}
	}
}
