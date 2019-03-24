package main

import (
	"context"
	"github.com/megaredfan/rpc-demo/client"
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/registry/zookeeper"
	"github.com/megaredfan/rpc-demo/server"
	"github.com/megaredfan/rpc-demo/service"
	"log"
	"math/rand"
	"strconv"
	"time"
)

const callTimes = 1

var s server.RPCServer

func main() {
	StartServer()
	time.Sleep(1e15)
	//start := time.Now()
	//for i := 0; i < callTimes; i++ {
	//	MakeCall(codec.GOB)
	//	//MakeCall(codec.MessagePack)
	//}
	//cost := time.Now().Sub(start)
	//log.Printf("cost:%s", cost)
	StopServer()
}

func StopServer() {
	s.Close()
}

var Registry = zookeeper.NewZookeeperRegistry("my-app", "/mns/sankuai/service",
	[]string{"127.0.0.1:2181"}, 1e10, nil)

func StartServer() {
	go func() {
		serverOpt := server.DefaultOption
		serverOpt.RegisterOption.AppKey = "my-app"
		serverOpt.Registry = Registry
		s = server.NewRPCServer(serverOpt)
		err := s.Register(service.Arith{}, make(map[string]string))
		if err != nil {
			log.Println("err!!!" + err.Error())
		}
		port := 8880
		s.Serve("tcp", ":"+strconv.Itoa(port))
	}()
}

func MakeCall(t codec.SerializeType) {
	op := &client.DefaultSGOption
	op.AppKey = "my-app"
	op.SerializeType = t
	op.RequestTimeout = time.Millisecond * 100
	op.DialTimeout = time.Millisecond * 100
	op.FailMode = client.FailRetry
	op.Retries = 3

	r := registry.NewPeer2PeerRegistry()
	r.Register(registry.RegisterOption{}, registry.Provider{ProviderKey: "tcp@:8880", Network: "tcp", Addr: ":8880"})
	op.Registry = r

	c := client.NewSGClient(*op)

	args := service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply := &service.Reply{}
	ctx := context.Background()
	err := c.Call(ctx, "Arith.Add", args, reply)
	if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A+args.B {
		log.Printf("%d + %d != %d", args.A, args.B, reply.C)
	}

	args = service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply = &service.Reply{}
	ctx = context.Background()
	err = c.Call(ctx, "Arith.Minus", args, reply)
	if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A-args.B {
		log.Printf("%d - %d != %d", args.A, args.B, reply.C)
	}

	args = service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply = &service.Reply{}
	ctx = context.Background()
	err = c.Call(ctx, "Arith.Mul", args, reply)
	if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A*args.B {
		log.Printf("%d * %d != %d", args.A, args.B, reply.C)
	}

	args = service.Args{A: rand.Intn(200), B: rand.Intn(100)}
	reply = &service.Reply{}
	ctx = context.Background()
	err = c.Call(ctx, "Arith.Divide", args, reply)
	if args.B == 0 && err == nil {
		log.Println("err!!! didn't return errror!")
	} else if err != nil && err.Error() == "divided by 0" {
		//log.Println(err.Error())
	} else if err != nil {
		log.Println("err!!!" + err.Error())
	} else if reply.C != args.A/args.B {
		log.Printf("%d / %d != %d", args.A, args.B, reply.C)
	}
}
