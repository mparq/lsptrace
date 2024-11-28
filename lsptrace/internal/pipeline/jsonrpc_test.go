package pipeline

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// TODO: all these tests are not the cleanest while I figure out go
// make testing from file simpler or go to in-memory approach
// also, actually start testing rather than just "smoke testing"
func TestJsonRpcStage(t *testing.T) {

	in := make(chan []byte)
	i := NewJsonRpcStage()
	f, err := os.Open("../testdata/test_reqres.out")
	if err != nil {
		t.Fatalf("Could not load test data.")
	}
	defer f.Close()

	out := i.Run(in)

	go func() {
		buf := make([]byte, 1024*8)
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			t.Logf("error in read %s", err)
		}
		if n < cap(buf) {
			in <- buf[:n]
		}
		close(in)
	}()
	for msg := range out {
		t.Log(msg)
	}
}

func TestChunkedReads(t *testing.T) {
	in := make(chan []byte)
	i := NewJsonRpcStage()
	out := i.Run(in)

	go func() {

		chunks := []string{
			"Content-Length: 146\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":62",
			",\"method\":\"textDocument/codeLens\",\"params\":{\"textDocument\":{\"uri\":\"file:///Users/mparq/code/vocabdex_blazor/Program.cs\"}}}Content-L",
			"ength: 37\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":62,\"result\":[]}",
		}
		for _, chunk := range chunks {
			in <- []byte(chunk)
		}
		close(in)
	}()
	for msg := range out {
		t.Log(msg)
	}
}

func TestRaw(t *testing.T) {
	in := make(chan []byte)
	i := NewJsonRpcStage()
	out := i.Run(in)
	f, _ := os.Open("../testdata/client.raw")
	go func() {
		defer close(in)
		buf := make([]byte, 8096)
		s := 0
		for {
			nr, err := f.Read(buf[s:])
			if err != nil {
				break
			}
			if nr > 0 {
				e := s + nr
				cloneBuf := bytes.Clone(buf[s:e])
				in <- cloneBuf
				if e >= len(buf) {
					s = 0
				} else {
					s = e
				}
			}
		}
	}()
	for msg := range out {
		t.Logf("jsonrpcrawtest: %s", msg)
	}
}
