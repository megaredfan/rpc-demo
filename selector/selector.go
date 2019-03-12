package selector

import (
	"context"
	"github.com/megaredfan/rpc-demo/client"
)

type Selector interface {
	Next(ctx context.Context, ServiceMethod string, arg interface{}) (client.RPCClient, error)
}
