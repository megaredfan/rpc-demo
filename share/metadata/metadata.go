package metadata

import (
	"context"
	"github.com/megaredfan/rpc-demo/protocol"
)

func FromContext(ctx context.Context) map[string]interface{} {
	metaData, ok := ctx.Value(protocol.MetaDataKey).(map[string]interface{})

	if !ok || metaData == nil {
		metaData = make(map[string]interface{})
	}
	return metaData
}

func WithMeta(ctx context.Context, meta map[string]interface{}) context.Context {
	return context.WithValue(ctx, protocol.MetaDataKey, meta)
}
