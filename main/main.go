package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/megaredfan/rpc-demo/client"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/server"
	"log"
	"math/rand"
	"sync"
	"time"
)

func main() {
	go func() {
		s := server.NewSimpleServer(server.DefaultOption)
		err := s.Register(Arith{}, make(map[string]string))
		if err != nil {
			log.Println("err!!!" + err.Error())
		}
		err = s.Serve("tcp", ":8881")
		if err != nil {
			log.Println("err!!!" + err.Error())
		}
	}()

	time.Sleep(1e9)

	wg := new(sync.WaitGroup)
	wg.Add(100)

	success := 0
	fail := 0
	for i := 0; i < 100; i++ {
		//go func() {
		//
		//}()
		op := &client.DefaultSGOption
		op.FailMode = client.FailRetry
		op.Retries = 1
		op.Registry = registry.NewPeer2PeerRegistry().WithProviders([]registry.Provider{
			{
				ProviderKey: "tcp@:8881",
				Network:     "tcp",
				Addr:        ":8881",
			},
		})

		c := client.NewSGClient(*op)

		args := Args{A: rand.Intn(200), B: rand.Intn(100)}
		log.Printf("=========== call %d Add %+v ============\n", i, args)
		reply := &Reply{}
		err := c.Call(context.TODO(), "Arith.Add", args, reply)
		if err != nil {
			log.Println("err!!!" + err.Error())
			fail++
		} else if reply.C != args.A+args.B {
			log.Println(reply.C)
			fail++
		} else {
			fmt.Println(reply.C)
			success++
		}

		log.Printf("=========== call %d Minus %+v ============\n", i, args)
		err = c.Call(context.TODO(), "Arith.Minus", args, reply)
		if err != nil {
			log.Println("err!!!" + err.Error())
			fail++
		} else if reply.C != args.A-args.B {
			log.Println(reply.C)
			fail++
		} else {
			fmt.Println(reply.C)
			success++
		}

		log.Printf("=========== call %d Mul %+v ============\n", i, args)
		err = c.Call(context.TODO(), "Arith.Mul", args, reply)
		if err != nil {
			log.Println("err!!!" + err.Error())
			fail++
		} else if reply.C != args.A*args.B {
			log.Println(reply.C)
			fail++
		} else {
			fmt.Println(reply.C)
			success++
		}

		log.Printf("=========== call %d Divide %+v ============\n", i, args)
		err = c.Call(context.TODO(), "Arith.Divide", args, reply)
		if args.B == 0 && err == nil {
			log.Println("err!!! didn't return errror!")
			fail++
		} else if err != nil {
			log.Println("err!!!" + err.Error())
			fail++
		} else if err != nil && err.Error() == "divided by 0" {
			log.Println(err.Error())
			success++
		} else if reply.C != args.A/args.B {
			log.Println(reply.C)
			fail++
		} else {
			fmt.Println(reply.C)
			success++
		}
		wg.Done()
	}
	wg.Wait()
	fmt.Printf("success:%d, fail:%d, success rate:%f%%", success, fail, float64(success)/float64(success+fail)*100)
}

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
