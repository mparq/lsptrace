package parse

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

func (t *LSPTracer) Parse(c chan *RawLSPMessage, out chan LSPTrace) {
	lspTrace := new(LSPTrace)
	for msg := range c {
		lspTrace.FromRaw(msg, t.from)
		switch lspTrace.MessageKind {
		case "request":
			t.saveRequestMethod(*lspTrace)
		case "response":
			method := t.popRequestMethod(*lspTrace)
			lspTrace.Method = &method
		}
		out <- *lspTrace
	}
	// close out channel if in channel was closed
	close(out)
}

func (t *LSPTracer) saveRequestMethod(trace LSPTrace) {
	t.reqMap.Insert(*trace.Id, *trace.Method)
}

func (t *LSPTracer) popRequestMethod(trace LSPTrace) string {
	return t.reqMap.Pop(*trace.Id)
}
