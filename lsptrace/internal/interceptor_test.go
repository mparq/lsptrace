package internal

import (
	"io"
	"os"
	"testing"
)

func TestInterceptor(t *testing.T) {

	in := make(chan []byte)
	out := make(chan *RawLSPMessage)
	i := NewInterceptor()
	i.In(in)
	i.Out(out)
	f, err := os.Open("testdata/test_reqres.out")
	if err != nil {
		t.Fatalf("Could not load test data.")
	}
	defer f.Close()
	go func() {
		buf := make([]byte, 1024)
		s := 0
		for {
			n, err := f.Read(buf[s:])
			if err == io.EOF {
				break
			}
			if err != nil || s+n > len(buf) {
				t.Logf("Error in read %s", err)
			}
			if n > 0 {
				in <- buf[s : s+n]
				s = s + n
				if s == len(buf) {
					s = 0
				}
			}
		}
		_, err := io.Copy(i, f)
		if err != nil {
			t.Log(err)
		}
		// close channel
		close(out)
	}()
	for msg := range out {
		t.Log(msg)
	}
}
