package server

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/share/metadata"
	"github.com/megaredfan/rpc-demo/share/trace"
	"github.com/megaredfan/rpc-demo/transport"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"log"
)

type OpenTracingInterceptor struct {
	defaultServerInterceptor
}

func (*OpenTracingInterceptor) WrapHandleRequest(s *SGServer, requestFunc HandleRequestFunc) HandleRequestFunc {
	return func(ctx context.Context, request *protocol.Message, response *protocol.Message, tr transport.Transport) {
		if protocol.MessageTypeHeartbeat != request.MessageType {
			meta := metadata.FromContext(ctx)
			spanContext, err := opentracing.GlobalTracer().Extract(opentracing.TextMap, &trace.MetaDataCarrier{&meta})
			if err != nil && err != opentracing.ErrSpanContextNotFound {
				log.Printf("extract span from meta error: %v", err)
			}

			serverSpan := opentracing.StartSpan(
				request.ServiceName+"."+request.MethodName,
				ext.RPCServerOption(spanContext),
				ext.SpanKindRPCServer)
			defer serverSpan.Finish()
			ctx = opentracing.ContextWithSpan(ctx, serverSpan)

		}
		requestFunc(ctx, request, response, tr)
	}
}
