package internal

import (
	"encoding/json"
	"testing"
)

func TestParse(t *testing.T) {
	c := make(chan *RawLSPMessage)
	tc := make(chan LSPTrace)

	tracer := NewLSPTracer("client", NewRequestMap())
	tracer.In(c)
	tracer.Out(tc)
	go tracer.Run()
	go func() {
		id := int64(64)
		method := "initialize"
		c <- &RawLSPMessage{Id: &id, Method: &method, Params: json.RawMessage{}}
		close(c)
	}()
	for trace := range tc {
		t.Log(trace)
	}
}

func TestParseReqResponse(t *testing.T) {
	c := make(chan *RawLSPMessage)
	tc := make(chan LSPTrace)
	sc := make(chan *RawLSPMessage)
	stc := make(chan LSPTrace)

	reqMap := NewRequestMap()
	tracer := NewLSPTracer("client", reqMap)
	tracer2 := NewLSPTracer("server", reqMap)
	tracer.In(c)
	tracer.Out(tc)
	tracer2.In(sc)
	tracer2.Out(stc)
	go tracer.Run()
	go tracer2.Run()
	go func() {
		id := int64(64)
		method := "initialize"
		c <- &RawLSPMessage{Id: &id, Method: &method, Params: json.RawMessage{}}
		id = int64(64)
		sc <- &RawLSPMessage{Id: &id, Result: json.RawMessage{}}
		close(c)
		close(sc)
	}()
	clientTrace := <-tc
	serverTrace := <-stc
	t.Log(clientTrace)
	t.Log(serverTrace)
	if *clientTrace.Method != *serverTrace.Method {
		t.Fatal("Method should be matched on response trace to the corresponding request trace.")
	}

}
