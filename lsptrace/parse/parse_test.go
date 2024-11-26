package parse

import (
	"encoding/json"
	"testing"
)

func TestParse(t *testing.T) {
	c := make(chan *RawLSPMessage)
	tc := make(chan LSPTrace)

	tracer := NewLSPTracer("client", NewRequestMap())
	go tracer.Parse(c, tc)
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
	go tracer.Parse(c, tc)
	go tracer2.Parse(sc, stc)
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
