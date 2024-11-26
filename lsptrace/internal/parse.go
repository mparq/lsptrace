package internal

import "log"

type LSPTracer struct {
	from   string
	reqMap *RequestMap
	in     chan *RawLSPMessage
	out    chan LSPTrace
}

func NewLSPTracer(sentFrom string, reqMap *RequestMap) *LSPTracer {
	if len(sentFrom) <= 0 {
		panic("assert: lsp tracer must specify 'sentFrom' source.")
	}
	return &LSPTracer{from: sentFrom, reqMap: reqMap}
}

func (t *LSPTracer) In(in chan *RawLSPMessage) {
	t.in = in
}

func (t *LSPTracer) Out(out chan LSPTrace) {
	t.out = out
}

func (t *LSPTracer) Run() {
	if t.in == nil || t.out == nil {
		panic("lsptracer: In and Out must be called before calling Run")
	}
	for msg := range t.in {
		log.Println("lsptracer: msg received from in channel")
		lspTrace := new(LSPTrace)
		lspTrace.FromRaw(msg, t.from)
		switch lspTrace.MessageKind {
		case "request":
			t.saveRequestMethod(*lspTrace)
		case "response":
			method := t.popRequestMethod(*lspTrace)
			lspTrace.Method = &method
		}
		log.Println("lsptracer: sending lsptrace method to out channel")
		t.out <- *lspTrace
	}
	// close out channel if in channel was closed
	close(t.out)
}

func (t *LSPTracer) saveRequestMethod(trace LSPTrace) {
	t.reqMap.Insert(*trace.Id, *trace.Method)
}

func (t *LSPTracer) popRequestMethod(trace LSPTrace) string {
	return t.reqMap.Pop(*trace.Id)
}
