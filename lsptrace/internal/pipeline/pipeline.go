package pipeline

import (
	"encoding/json"
	"io"
	"log"
	"lsptrace/internal"
	// "lsptrace/internal/channel"
)

type Pipeline struct {
	// inputs
	rawIn    io.Reader
	rawOut   io.Writer
	traceOut io.Writer
	// the work node which has an input channel expecting raw jsonrpc message
	// and output channel which it will send processed LSPTrace items to
	lspTracer internal.LSPTracer
}

func NewPipeline(rawIn io.Reader, rawOut io.Writer, traceOut io.Writer, lspTracer internal.LSPTracer) *Pipeline {
	return &Pipeline{rawIn, rawOut, traceOut, lspTracer}
}

func (p *Pipeline) Run() (done chan int) {
	inputOut := RunInputStage(p.rawIn, p.rawOut)
	jsonRpcOut := RunJsonRpcStage(inputOut)
	runTraceOut := RunLSPTraceStage(jsonRpcOut, p.lspTracer)
	done = RunOutputStage(runTraceOut, p.traceOut)
	return done
}

// InputStage: streams data from raw input to raw output and the returned out channel
func RunInputStage(rawIn io.Reader, rawOut io.Writer) chan []byte {
	out := make(chan []byte)
	// cw := channel.NewWriter(out)
	// do work
	go func() {
		buf := make([]byte, 32*1024)
		// TODO: channel writer is choking the copy
		for {
			nr, err := rawIn.Read(buf)
			if err != nil {
				log.Printf("inputstage: err on read %s", err)
				break
			}
			rawOut.Write(buf[:nr])
			out <- buf[:nr]
		}
		// close out channel after copy EOF
		close(out)
	}()
	return out
}

func RunJsonRpcStage(in chan []byte) chan *internal.RawLSPMessage {
	jsonRpcStage := NewJsonRpcStage()
	return jsonRpcStage.Run(in)
}

func RunLSPTraceStage(in chan *internal.RawLSPMessage, lspTracer internal.LSPTracer) chan *internal.LSPTrace {
	out := make(chan *internal.LSPTrace)
	// do work
	go func() {
		for jsonrpc := range in {
			log.Printf("lsptracestage: jsonrpc message received: %s\n", jsonrpc)
			trace := lspTracer.MakeTrace(jsonrpc)
			out <- trace
		}
		close(out)
	}()
	return out
}

func RunOutputStage(in chan *internal.LSPTrace, out io.Writer) (done chan int) {
	done = make(chan int)
	// do work
	go func() {
		for trace := range in {
			log.Printf("outputstage: writing output trace\n")
			traceJson, err := json.Marshal(trace)
			if err != nil {
				// TODO: handle err
				log.Println("pipeline: output stage: Unexpected error marshalling lsp trace.")
				break
			}
			out.Write(append(traceJson, '\n'))
		}
		done <- 1
	}()
	return done
}
