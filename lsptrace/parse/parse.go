package parse

import (
	"sync"
)

type LSPTracer struct {
	from   string
	rMutex sync.Mutex
	// map from request id to method to map corresponding responses
	rMap map[int]string
}

func NewLSPTracer(sentFrom string) *LSPTracer {
	if len(sentFrom) <= 0 {
		panic("assert: lsp tracer must specify 'sentFrom' source.")
	}
	return &LSPTracer{from: sentFrom, rMap: make(map[int]string)}
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
	t.rMutex.Lock()
	t.rMap[int(*trace.Id)] = *trace.Method
	t.rMutex.Unlock()
}

func (t *LSPTracer) popRequestMethod(trace LSPTrace) string {
	t.rMutex.Lock()
	method := t.rMap[int(*trace.Id)]
	delete(t.rMap, int(*trace.Id))
	t.rMutex.Unlock()
	return method
}
