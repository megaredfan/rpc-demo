package codec

import (
	"testing"
	"time"
)

type Header struct {
	Seq           uint64                 //序号, 用来唯一标识请求或响应
	MessageType   byte                   //消息类型，用来标识一个消息是请求还是响应
	CompressType  byte                   //压缩类型，用来标识一个消息的压缩方式
	SerializeType SerializeType          //序列化类型，用来标识消息体采用的编码方式
	StatusCode    byte                   //状态类型，用来标识一个请求是正常还是异常
	ServiceName   string                 //服务名
	MethodName    string                 //方法名
	Error         string                 //方法调用发生的异常
	MetaData      map[string]interface{} //其他元数据
}

func BenchmarkMessagePackCodec(b *testing.B) {
	h := Header{
		Seq:           1,
		MessageType:   0,
		CompressType:  0,
		SerializeType: MessagePack,
		StatusCode:    0,
		ServiceName:   "Arith",
		MethodName:    "Add",
		Error:         "divided by 0",
		MetaData: map[string]interface{}{"rpc_request_seq": 1,
			"rpc_request_timeout":  time.Millisecond * 500,
			"rpc_request_deadline": time.Now().Add(time.Millisecond * 500)},
	}
	gobc := &GobCodec{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := gobc.Encode(h)
		gobc.Decode(data, h)
	}
}

func BenchmarkGetCodec(b *testing.B) {
	h := Header{
		Seq:           1,
		MessageType:   0,
		CompressType:  0,
		SerializeType: MessagePack,
		StatusCode:    0,
		ServiceName:   "Arith",
		MethodName:    "Add",
		Error:         "divided by 0",
		MetaData: map[string]interface{}{"rpc_request_seq": 1,
			"rpc_request_timeout":  time.Millisecond * 500,
			"rpc_request_deadline": time.Now().Add(time.Millisecond * 500)},
	}
	msgpc := &MessagePackCodec{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := msgpc.Encode(h)
		msgpc.Decode(data, h)
	}
}
