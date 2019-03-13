package selector

import (
	"context"
	"errors"
	"github.com/megaredfan/rpc-demo/registry"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var ErrEmptyProviderList = errors.New("provider list is empty")

type Filter func(provider registry.Provider, ctx context.Context, ServiceMethod string, arg interface{}) bool

type SelectOption struct {
	Filter Filter
}

type Selector interface {
	Next(providers []registry.Provider, ctx context.Context, ServiceMethod string, arg interface{}, opts ...SelectOption) (registry.Provider, error)
}

type RandomSelector struct {
}

var RandomSelectorInstance = RandomSelector{}

func (RandomSelector) Next(providers []registry.Provider, ctx context.Context, ServiceMethod string, arg interface{}, opts ...SelectOption) (p registry.Provider, err error) {
	filters := combineFilter(opts)
	list := make([]registry.Provider, 0)
	for _, p := range providers {
		if filters(p, ctx, ServiceMethod, arg) {
			list = append(list, p)
		}
	}

	if len(list) == 0 {
		err = ErrEmptyProviderList
		return
	}
	i := rand.Intn(len(list))
	p = list[i]
	return
}

func combineFilter(options []SelectOption) Filter {
	return func(provider registry.Provider, ctx context.Context, ServiceMethod string, arg interface{}) bool {
		for _, op := range options {
			if !op.Filter(provider, ctx, ServiceMethod, arg) {
				return false
			}
		}
		return true
	}
}

func NewRandomSelector() Selector {
	return RandomSelectorInstance
}
