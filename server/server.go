package server

import (
	"context"
	"errors"
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/protocol"
	"github.com/megaredfan/rpc-demo/registry"
	"github.com/megaredfan/rpc-demo/transport"
	"io"
	"log"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type RPCServer interface {
	Register(rcvr interface{}, metaData map[string]string) error
	Serve(network string, addr string) error
	Close() error
}

type simpleServer struct {
	codec      codec.Codec
	serviceMap sync.Map
	tr         transport.ServerTransport
	mutex      sync.Mutex
	shutdown   bool

	option Option
}

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
}

type service struct {
	name    string
	typ     reflect.Type
	rcvr    reflect.Value
	methods map[string]*methodType
}

func NewSimpleServer(option Option) RPCServer {
	s := new(simpleServer)
	s.option = option
	s.codec = codec.GetCodec(option.SerializeType)
	return s
}

func (s *simpleServer) Register(rcvr interface{}, metaData map[string]string) error {
	typ := reflect.TypeOf(rcvr)
	name := typ.Name()
	srv := new(service)
	srv.name = name
	srv.rcvr = reflect.ValueOf(rcvr)
	srv.typ = typ
	methods := suitableMethods(typ, true)
	srv.methods = methods

	if len(srv.methods) == 0 {
		var errorStr string

		// 如果对应的类型没有任何符合规则的方法，扫描对应的指针类型
		// 也是从net.rpc包里抄来的
		method := suitableMethods(reflect.PtrTo(srv.typ), false)
		if len(method) != 0 {
			errorStr = "rpcx.Register: type " + name + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			errorStr = "rpcx.Register: type " + name + " has no exported methods of suitable type"
		}
		log.Println(errorStr)
		return errors.New(errorStr)
	}
	if _, duplicate := s.serviceMap.LoadOrStore(name, srv); duplicate {
		return errors.New("rpc: service already defined: " + name)
	}
	return nil
}

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()
var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

//过滤符合规则的方法，从net.rpc包抄的
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name

		// 方法必须是可导出的
		if method.PkgPath != "" {
			continue
		}
		// 需要有四个参数: receiver, Context, args, *reply.
		if mtype.NumIn() != 4 {
			if reportErr {
				log.Println("method", mname, "has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// 第一个参数必须是context.Context
		ctxType := mtype.In(1)
		if !ctxType.Implements(typeOfContext) {
			if reportErr {
				log.Println("method", mname, " must use context.Context as the first parameter")
			}
			continue
		}

		// 第二个参数是arg
		argType := mtype.In(2)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				log.Println(mname, "parameter type not exported:", argType)
			}
			continue
		}
		// 第三个参数是返回值，必须是指针类型的
		replyType := mtype.In(3)
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				log.Println("method", mname, "reply type not a pointer:", replyType)
			}
			continue
		}
		// 返回值的类型必须是可导出的
		if !isExportedOrBuiltinType(replyType) {
			if reportErr {
				log.Println("method", mname, "reply type not exported:", replyType)
			}
			continue
		}
		// 必须有一个返回值
		if mtype.NumOut() != 1 {
			if reportErr {
				log.Println("method", mname, "has wrong number of outs:", mtype.NumOut())
			}
			continue
		}
		// 返回值类型必须是error
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				log.Println("method", mname, "returns", returnType.String(), "not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func (s *simpleServer) Serve(network string, addr string) error {
	provider := registry.Provider{
		ProviderKey: network + "@" + addr,
		Network:     network,
		Addr:        addr,
	}
	s.option.Registry.Register(s.option.ServiceKey, provider)
	log.Printf("registered provider %v", provider)

	s.tr = transport.NewServerTransport(s.option.TransportType)
	err := s.tr.Listen(network, addr)
	if err != nil {
		log.Println(err)
		return err
	}
	for {
		conn, err := s.tr.Accept()
		if err != nil {
			log.Println(err)
			return err
		}
		go s.serveTransport(conn)
	}

}

func (s *simpleServer) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.shutdown = true

	err := s.tr.Close()

	s.serviceMap.Range(func(key, value interface{}) bool {
		s.serviceMap.Delete(key)
		return true
	})
	return err
}

type Request struct {
	Seq   uint32
	Reply interface{}
	Data  []byte
}

func (s *simpleServer) serveTransport(tr transport.Transport) {
	for {
		if rand.Intn(3) == 0 {
			log.Printf("randomly close transport: %s-> %s", tr.RemoteAddr(), tr.LocalAddr())
			tr.Close()
			return
		}
		request, err := protocol.DecodeMessage(s.option.ProtocolType, tr)

		if err != nil {
			if err == io.EOF {
				log.Printf("client has closed this connection: %s", tr.RemoteAddr().String())
			} else if strings.Contains(err.Error(), "use of closed network connection") {
				log.Printf("rpcx: connection %s is closed", tr.RemoteAddr().String())
			} else {
				log.Printf("rpcx: failed to read request: %v", err)
			}
			return
		}
		response := request.Clone()
		response.MessageType = protocol.MessageTypeResponse

		sname := request.ServiceName
		mname := request.MethodName
		srvInterface, ok := s.serviceMap.Load(sname)
		if !ok {
			s.writeErrorResponse(response, tr, "can not find service")
			return
		}
		srv, ok := srvInterface.(*service)
		if !ok {
			s.writeErrorResponse(response, tr, "not *service type")
			return

		}

		mtype, ok := srv.methods[mname]
		if !ok {
			s.writeErrorResponse(response, tr, "can not find method")
			return
		}
		argv := newValue(mtype.ArgType)
		replyv := newValue(mtype.ReplyType)

		ctx := context.Background()
		err = s.codec.Decode(request.Data, argv)

		var returns []reflect.Value
		if mtype.ArgType.Kind() != reflect.Ptr {
			returns = mtype.method.Func.Call([]reflect.Value{srv.rcvr,
				reflect.ValueOf(ctx),
				reflect.ValueOf(argv).Elem(),
				reflect.ValueOf(replyv)})
		} else {
			returns = mtype.method.Func.Call([]reflect.Value{srv.rcvr,
				reflect.ValueOf(ctx),
				reflect.ValueOf(argv),
				reflect.ValueOf(replyv)})
		}
		if len(returns) > 0 && returns[0].Interface() != nil {
			err = returns[0].Interface().(error)
			s.writeErrorResponse(response, tr, err.Error())
			return
		}

		responseData, err := codec.GetCodec(request.SerializeType).Encode(replyv)
		if err != nil {
			s.writeErrorResponse(response, tr, err.Error())
			return
		}

		response.StatusCode = protocol.StatusOK
		response.Data = responseData

		_, err = tr.Write(protocol.EncodeMessage(s.option.ProtocolType, response))
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func newValue(t reflect.Type) interface{} {
	if t.Kind() == reflect.Ptr {
		return reflect.New(t.Elem()).Interface()
	} else {
		return reflect.New(t).Interface()
	}
}

func (s *simpleServer) writeErrorResponse(response *protocol.Message, w io.Writer, err string) {
	response.Error = err
	log.Println(response.Error)
	response.StatusCode = protocol.StatusError
	response.Data = response.Data[:0]
	_, _ = w.Write(protocol.EncodeMessage(s.option.ProtocolType, response))
}
