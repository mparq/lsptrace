package internal

import (
	"encoding/json"
	"testing"
)

func TestParse(t *testing.T) {

	tracer := NewLSPTracer(NewRequestMap())
	id := int64(64)
	method := "initialize"
	msg := &RawLSPMessage{Id: &id, Method: &method, Params: json.RawMessage{}}
	actual := tracer.MakeTrace(msg, "client")
	t.Logf("actual: %s\n", actual)

}

func TestParseReqResponse(t *testing.T) {
	reqMap := NewRequestMap()
	tracer := NewLSPTracer(reqMap)
	id := new(int64)
	*id = 64
	var method *string
	method = new(string)
	*method = "initialize"
	t.Logf("addr of method: %v", method)
	clientTrace := tracer.MakeTrace(&RawLSPMessage{Id: id, Method: method, Params: json.RawMessage{}}, "client")
	id = new(int64)
	*id = 70
	method = new(string)
	*method = "other-method"
	t.Logf("addr of method after re-assign: %v", method)
	otherTrace := tracer.MakeTrace(&RawLSPMessage{Id: id, Method: method, Params: []byte("{}")}, "client")
	id = new(int64)
	*id = int64(64)
	serverTrace := tracer.MakeTrace(&RawLSPMessage{Id: id, Result: json.RawMessage{}}, "server")
	t.Log(clientTrace)
	t.Log(otherTrace)
	t.Log(serverTrace)
	if *clientTrace.Method != *serverTrace.Method {
		t.Fatal("Method should be matched on response trace to the corresponding request trace.")
	}

}
