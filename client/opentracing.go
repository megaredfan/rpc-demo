package client

import (
	"context"
	"github.com/megaredfan/rpc-demo/share/metadata"
	"github.com/megaredfan/rpc-demo/share/trace"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	opentracingLog "github.com/opentracing/opentracing-go/log"
	"log"
)

type OpenTracingInterceptor struct {
	defaultClientInterceptor
}

func (*OpenTracingInterceptor) WrapCall(option *SGOption, callFunc CallFunc) CallFunc {
	return func(ctx context.Context, ServiceMethod string, arg interface{}, reply interface{}) error {
		var clientSpan opentracing.Span
		if ServiceMethod != "" {
			var parentCtx opentracing.SpanContext
			if parent := opentracing.SpanFromContext(ctx); parent != nil {
				parentCtx = parent.Context()
			}
			clientSpan := opentracing.StartSpan(
				ServiceMethod,
				opentracing.ChildOf(parentCtx),
				ext.SpanKindRPCClient)
			defer clientSpan.Finish()

			meta := metadata.FromContext(ctx)
			writer := &trace.MetaDataCarrier{&meta}

			injectErr := opentracing.GlobalTracer().Inject(clientSpan.Context(), opentracing.TextMap, writer)
			if injectErr != nil {
				log.Printf("inject trace error: %v", injectErr)
			}
			ctx = metadata.WithMeta(ctx, meta)
		}

		err := callFunc(ctx, ServiceMethod, arg, reply)
		if err != nil && clientSpan != nil {
			clientSpan.LogFields(opentracingLog.String("error", err.Error()))
		}
		return err
	}
}
