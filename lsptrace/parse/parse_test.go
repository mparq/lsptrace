package parse

import (
	"encoding/json"
	"testing"
)

func TestParse(t *testing.T) {
	c := make(chan *RawLSPMessage)
	tc := make(chan LSPTrace)

	tracer := NewLSPTracer("client")
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

	tracer := NewLSPTracer("client")
	go tracer.Parse(c, tc)
	go func() {
		id := int64(64)
		method := "initialize"
		c <- &RawLSPMessage{Id: &id, Method: &method, Params: json.RawMessage{}}
		id = int64(64)
		c <- &RawLSPMessage{Id: &id, Result: json.RawMessage{}}
		close(c)
	}()
	for trace := range tc {
		t.Log(trace)
	}
}
