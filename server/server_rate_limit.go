package server

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/share/ratelimit"
	"github.com/megaredfan/rpc-demo/transport"
)

type RequestRateLimitInterceptor struct {
	defaultServerInterceptor
	Limiter ratelimit.RateLimiter
}

func (rl *RequestRateLimitInterceptor) WrapHandleRequest(s *SGServer, requestFunc HandleRequestFunc) HandleRequestFunc {
	return func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport) {
		if rl.Limiter != nil {
			if rl.Limiter.TryAcquire() {
				requestFunc(ctx, request, response, tr)
			} else {
				s.writeErrorResponse(response, tr, "request limited")
			}
		} else {
			requestFunc(ctx, request, response, tr)
		}
	}
}
