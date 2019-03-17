package service

import (
	"context"
	"errors"
)

type Arith struct{}

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

//arg可以是指针类型，也可以是指针类型
func (a Arith) Add(ctx context.Context, arg *Args, reply *Reply) error {
	reply.C = arg.A + arg.B
	return nil
}

func (a Arith) Minus(ctx context.Context, arg Args, reply *Reply) error {
	reply.C = arg.A - arg.B
	return nil
}

func (a Arith) Mul(ctx context.Context, arg Args, reply *Reply) error {
	reply.C = arg.A * arg.B
	return nil
}

func (a Arith) Divide(ctx context.Context, arg *Args, reply *Reply) error {
	if arg.B == 0 {
		return errors.New("divided by 0")
	}
	reply.C = arg.A / arg.B
	return nil
}
