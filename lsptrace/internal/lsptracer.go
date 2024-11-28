package internal

import "log"

type LSPTracer struct {
	clientReqMap *RequestMap
	serverReqMap *RequestMap
}

func NewLSPTracer(reqMap *RequestMap) *LSPTracer {
	clientReqMap := NewRequestMap()
	serverReqMap := NewRequestMap()
	return &LSPTracer{clientReqMap, serverReqMap}
}

func (t *LSPTracer) MakeTrace(msg *RawLSPMessage, sentFrom string) (trace *LSPTrace) {
	if sentFrom != "client" && sentFrom != "server" {
		panic("assert: lsp tracer must specify valid 'sentFrom' source.")
	}
	log.Printf("lsptracer(%s): msg received from in channel\n", sentFrom)
	trace = new(LSPTrace)
	trace.FromRaw(msg, sentFrom)
	switch trace.MessageKind {
	case "request":
		t.saveRequestMethod(trace, sentFrom)
	case "response":
		method := t.popRequestMethod(trace, sentFrom)
		trace.Method = &method
	}
	log.Printf("lsptracer(%s): sending lsptrace method to out channel", sentFrom)
	return trace
}

func (t *LSPTracer) saveRequestMethod(trace *LSPTrace, sentFrom string) {
	if sentFrom == "client" {
		log.Printf("push to client reqmap: %v\n", *trace.Id)
		t.clientReqMap.PushRequest(*trace.Id, *trace.Method)
		log.Printf("%v %s\n", &t.clientReqMap, t.clientReqMap)
	} else {
		log.Printf("push to server reqmap: %v\n", *trace.Id)
		t.serverReqMap.PushRequest(*trace.Id, *trace.Method)
		log.Printf("%v %s\n", &t.serverReqMap, t.serverReqMap)
	}
}

func (t *LSPTracer) popRequestMethod(trace *LSPTrace, sentFrom string) string {
	if sentFrom == "client" {
		log.Printf("pop from server reqmap: %v\n", *trace.Id)
		log.Printf("%v %s\n", &t.serverReqMap, t.serverReqMap)
		return t.serverReqMap.Pop(*trace.Id)
	} else {
		log.Printf("pop from client reqmap: %v\n", *trace.Id)
		log.Printf("%v %s\n", &t.clientReqMap, t.clientReqMap)
		return t.clientReqMap.Pop(*trace.Id)
	}
}
