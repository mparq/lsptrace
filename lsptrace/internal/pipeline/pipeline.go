package pipeline

import (
	"bytes"
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
	// label representing the source of the pipeline (client | server)
	sentFrom string
	// the work node which has an input channel expecting raw jsonrpc message
	// and output channel which it will send processed LSPTrace items to
	lspTracer *internal.LSPTracer
}

func NewPipeline(rawIn io.Reader, rawOut io.Writer, traceOut io.Writer, lspTracer *internal.LSPTracer, sentFrom string) *Pipeline {
	log.Printf("%s pipeline lspTracer addr %v\n", sentFrom, lspTracer)
	return &Pipeline{rawIn, rawOut, traceOut, sentFrom, lspTracer}
}

func (p *Pipeline) Run() (done chan int) {
	inputOut, start := p.RunInputStage(p.rawIn, p.rawOut)
	jsonRpcOut := p.RunJsonRpcStage(inputOut)
	runTraceOut := p.RunLSPTraceStage(jsonRpcOut, p.lspTracer)
	done = p.RunOutputStage(runTraceOut, p.traceOut)
	// wait to hook up all channels and then send start signal to pipeline input stage
	start <- 1
	return done
}

// InputStage: streams data from raw input to raw output and the returned out channel
func (p *Pipeline) RunInputStage(rawIn io.Reader, rawOut io.Writer) (out chan []byte, start chan int) {
	// TODO: start is used to "wait" for the other stages to be set up to start the pipeline (reading from rawIn)
	// should be a cleaner way to do this
	start = make(chan int)
	out = make(chan []byte)
	// cw := channel.NewWriter(out)
	// do work
	go func() {
		defer close(out)
		<-start
		buf := make([]byte, 16*1024)
		s := 0
		// TODO: channel writer is choking the copy
		var err error
		for {
			var nr int
			nr, err = rawIn.Read(buf[s:])
			if err != nil {
				break
			}
			if nr > 0 {
				e := s + nr
				// TODO: error case
				// NOTE: do we need to clone here? or should consuming channels
				// be expected to block this?
				outClone := bytes.Clone(buf[s:e])
				rawOut.Write(outClone)
				out <- outClone
				if e >= len(buf) {
					s = 0
				} else {
					s = e
				}
			}
		}
		if err != io.EOF {
			log.Fatalf("error reading file %s", err)
		}
	}()
	return out, start
}

func (p *Pipeline) RunJsonRpcStage(in chan []byte) chan *internal.RawLSPMessage {
	jsonRpcStage := NewJsonRpcStage()
	return jsonRpcStage.Run(in)
}

func (p *Pipeline) RunLSPTraceStage(in chan *internal.RawLSPMessage, lspTracer *internal.LSPTracer) chan *internal.LSPTrace {
	out := make(chan *internal.LSPTrace)
	// do work
	go func() {
		for jsonrpc := range in {
			trace := lspTracer.MakeTrace(jsonrpc, p.sentFrom)
			out <- trace
		}
		close(out)
	}()
	return out
}

func (p *Pipeline) RunOutputStage(in chan *internal.LSPTrace, out io.Writer) (done chan int) {
	done = make(chan int)
	// do work
	go func() {
		for trace := range in {
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
