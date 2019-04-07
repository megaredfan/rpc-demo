package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/megaredfan/rpc-demo/codec"
	"github.com/megaredfan/rpc-demo/protocol"
	"log"
	"net/http"
	"testing"
)

func Test(t *testing.T) {
	arg := "hello world"
	data := bytes.NewBufferString(arg)
	req, _ := http.NewRequest("GET", "/", data)
	req.Header.Set(HEADER_SEQ, "1")
	req.Header.Set(HEADER_MESSAGE_TYPE, protocol.MessageTypeRequest.String())
	req.Header.Set(HEADER_COMPRESS_TYPE, protocol.CompressTypeNone.String())
	req.Header.Set(HEADER_SERIALIZE_TYPE, codec.MessagePack.String())
	req.Header.Set(HEADER_STATUS_CODE, protocol.StatusOK.String())
	req.Header.Set(HEADER_SERVICE_NAME, "Hello")
	req.Header.Set(HEADER_METHOD_NAME, "World")
	req.Header.Set(HEADER_ERROR, "")
	meta := map[string]interface{}{"key": "value"}
	metaJson, _ := json.Marshal(meta)
	req.Header.Set(HEADER_META_DATA, string(metaJson))

	msg, err := parseHeader(&protocol.Message{Header: &protocol.Header{}}, req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(msg)
}
