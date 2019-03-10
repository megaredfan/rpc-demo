package codec

import (
	"github.com/vmihailenco/msgpack"
)

type SerializeType byte

const (
	MessagePack SerializeType = iota
)

var codecs = map[SerializeType]Codec{
	MessagePack: &MessagePackCodec{},
}

type Codec interface {
	Encode(value interface{}) ([]byte, error)
	Decode(data []byte, value interface{}) error
}

func GetCodec(t SerializeType) Codec {
	return codecs[t]
}

type MessagePackCodec struct{}

func (c MessagePackCodec) Encode(v interface{}) ([]byte, error) {
	return msgpack.Marshal(v)
}

func (c MessagePackCodec) Decode(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}
