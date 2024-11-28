package pipeline

import (
	"bytes"
	"lsptrace/internal"
	"os"
	"strings"
	"testing"
)

var (
	clientInput = strings.Join([]string{
		"Content-Length: 146",
		`{"jsonrpc":"2.0","id":62,"method":"textDocument/codeLens","params":{"textDocument":{"uri":"file:///Users/mparq/code/vocabdex_blazor/Program.cs"}}}`,
	}, "\r\n\r\n")
	serverInput = strings.Join([]string{
		`Content-Length: 37`,
		`{"jsonrpc":"2.0","id":62,"result":[]}`,
	}, "\r\n\r\n")
)

func TestPipeline(t *testing.T) {
	in := strings.NewReader(clientInput)
	out := new(bytes.Buffer)
	traceOut := new(bytes.Buffer)
	reqMap := internal.NewRequestMap()
	lspTracer := internal.NewLSPTracer(reqMap)
	p := NewPipeline(in, out, traceOut, lspTracer, "client")
	done := p.Run()
	<-done
	t.Logf("out: %s", string(out.Bytes()))
	t.Logf("trace: %s", string(traceOut.Bytes()))
}

func TestDoublePipeline(t *testing.T) {
	reqMap := internal.NewRequestMap()

	in := strings.NewReader(clientInput)
	out := new(bytes.Buffer)
	traceOut := new(bytes.Buffer)
	lspTracer := internal.NewLSPTracer(reqMap)
	p := NewPipeline(in, out, traceOut, lspTracer, "client")
	done := p.Run()

	serverIn := strings.NewReader(serverInput)
	sOut := new(bytes.Buffer)
	sTraceOut := new(bytes.Buffer)
	sp := NewPipeline(serverIn, sOut, sTraceOut, lspTracer, "server")
	sdone := sp.Run()

	go func() {
		<-sdone
		t.Logf("server out: %s", string(sOut.Bytes()))
		t.Logf("server trace: %s", string(sTraceOut.Bytes()))
	}()

	<-done
	t.Logf("client out: %s", string(out.Bytes()))
	t.Logf("client trace: %s", string(traceOut.Bytes()))
}

func TestBigPipeline(t *testing.T) {
	reqMap := internal.NewRequestMap()
	in, _ := os.Open("../testdata/client.raw")
	// TODO: don't write out file in test
	out, _ := os.OpenFile("../testdata/client_out.raw", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	trace, _ := os.OpenFile("../testdata/client_trace.raw", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	lspTracer := internal.NewLSPTracer(reqMap)
	p := NewPipeline(in, out, trace, lspTracer, "client")
	done := p.Run()
	<-done
}
