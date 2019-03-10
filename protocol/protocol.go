package protocol

import (
	"encoding/binary"
	"errors"
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/vmihailenco/msgpack"
	"io"
)

//-------------------------------------------------------------------------------------------------
//|2byte|1byte  |4byte       |4byte        | header length |(total length - header length - 4byte)|
//-------------------------------------------------------------------------------------------------
//|magic|version|total length|header length|     header    |                    body              |
//-------------------------------------------------------------------------------------------------

type MessageType byte

//请求类型
const (
	MessageTypeRequest MessageType = iota
	MessageTypeResponse
)

type CompressType byte

const (
	CompressTypeNone CompressType = iota
)

type StatusCode byte

const (
	StatusOK StatusCode = iota
	StatusError
)

type ProtocolType byte

const (
	Default ProtocolType = iota
)

type Protocol interface {
	NewMessage() *Message
	DecodeMessage(r io.Reader) (*Message, error)
	EncodeMessage(message *Message) []byte
}

var protocols = map[ProtocolType]Protocol{
	Default: &RPCProtocol{},
}

const (
	RequestSeqKey     = "rpc_request_seq"
	RequestTimeoutKey = "rpc_request_timeout"
	MetaDataKey       = "rpc_meta_data"
)

type Header struct {
	Seq           uint64              //序号, 用来唯一标识请求或响应
	MessageType   MessageType         //消息类型，用来标识一个消息是请求还是响应
	CompressType  CompressType        //压缩类型，用来标识一个消息的压缩方式
	SerializeType codec.SerializeType //序列化类型，用来标识消息体采用的编码方式
	StatusCode    StatusCode          //状态类型，用来标识一个请求是正常还是异常
	ServiceName   string              //服务名
	MethodName    string              //方法名
	Error         string              //方法调用发生的异常
	MetaData      map[string]string   //其他元数据
}

func NewMessage(t ProtocolType) *Message {
	return protocols[t].NewMessage()
}

func DecodeMessage(t ProtocolType, r io.Reader) (*Message, error) {
	return protocols[t].DecodeMessage(r)
}

func EncodeMessage(t ProtocolType, m *Message) []byte {
	return protocols[t].EncodeMessage(m)
}

type Message struct {
	*Header
	Data []byte
}

func (m Message) Clone() *Message {
	header := *m.Header
	c := new(Message)
	c.Header = &header
	c.Data = m.Data
	return c
}

type RPCProtocol struct {
}

func (RPCProtocol) NewMessage() *Message {
	return &Message{Header: &Header{}}
}

func (RPCProtocol) DecodeMessage(r io.Reader) (msg *Message, err error) {
	first3bytes := make([]byte, 3)
	_, err = io.ReadFull(r, first3bytes)
	if err != nil {
		return
	}
	if !checkMagic(first3bytes[:2]) {
		err = errors.New("wrong protocol")
		return
	}
	totalLenBytes := make([]byte, 4)
	_, err = io.ReadFull(r, totalLenBytes)
	if err != nil {
		return
	}
	totalLen := int(binary.BigEndian.Uint32(totalLenBytes))
	if totalLen < 4 {
		err = errors.New("invalid total length")
		return
	}
	data := make([]byte, totalLen)
	_, err = io.ReadFull(r, data)
	headerLen := int(binary.BigEndian.Uint32(data[:4]))
	headerBytes := data[4 : headerLen+4]
	header := &Header{}
	err = msgpack.Unmarshal(headerBytes, header)
	if err != nil {
		return
	}
	msg = new(Message)
	msg.Header = header
	msg.Data = data[headerLen+4:]
	return
}

func (RPCProtocol) EncodeMessage(msg *Message) []byte {
	first3bytes := []byte{0xab, 0xba, 0x00}
	headerBytes, _ := msgpack.Marshal(msg.Header)

	totalLen := 4 + len(headerBytes) + len(msg.Data)
	totalLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(totalLenBytes, uint32(totalLen))

	data := make([]byte, totalLen+7)
	start := 0
	copyFullWithOffset(data, first3bytes, &start)
	copyFullWithOffset(data, totalLenBytes, &start)

	headerLenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(headerLenBytes, uint32(len(headerBytes)))
	copyFullWithOffset(data, headerLenBytes, &start)
	copyFullWithOffset(data, headerBytes, &start)
	copyFullWithOffset(data, msg.Data, &start)
	return data
}

func checkMagic(bytes []byte) bool {
	return bytes[0] == 0xab && bytes[1] == 0xba
}

func copyFullWithOffset(dst []byte, src []byte, start *int) {
	copy(dst[*start:*start+len(src)], src)
	*start = *start + len(src)
}
