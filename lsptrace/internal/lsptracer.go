package internal

import "log"

type LSPTracer struct {
	from   string
	reqMap *RequestMap
}

func NewLSPTracer(sentFrom string, reqMap *RequestMap) *LSPTracer {
	if len(sentFrom) <= 0 {
		panic("assert: lsp tracer must specify 'sentFrom' source.")
	}
	return &LSPTracer{from: sentFrom, reqMap: reqMap}
}

func (t *LSPTracer) MakeTrace(msg *RawLSPMessage) (trace *LSPTrace) {
	log.Printf("lsptracer(%s): msg received from in channel\n", t.from)
	trace = new(LSPTrace)
	trace.FromRaw(msg, t.from)
	switch trace.MessageKind {
	case "request":
		t.saveRequestMethod(trace)
	case "response":
		method := t.popRequestMethod(trace)
		trace.Method = &method
	}
	log.Printf("lsptracer(%s): sending lsptrace method to out channel", t.from)
	return trace
}

func (t *LSPTracer) saveRequestMethod(trace *LSPTrace) {
	t.reqMap.Insert(*trace.Id, *trace.Method)
}

func (t *LSPTracer) popRequestMethod(trace *LSPTrace) string {
	return t.reqMap.Pop(*trace.Id)
}
