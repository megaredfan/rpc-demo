package main

import (
	"github.com/megaredfan/rpc-demo/codec"
	"testing"
)

func BenchmarkMakeCallGOB(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MakeCall(codec.GOB)
	}
}

func BenchmarkMakeCallMSGP(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MakeCall(codec.MessagePack)
	}
}
