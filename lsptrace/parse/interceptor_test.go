package parse

import (
	"io"
	"os"
	"testing"
)

func TestInterceptor(t *testing.T) {

	c := make(chan *RawLSPMessage)
	i := NewInterceptor(c)
	f, err := os.Open("testdata/test_reqres.out")
	if err != nil {
		t.Fatalf("Could not load test data.")
	}
	defer f.Close()
	go func() {
		_, err := io.Copy(i, f)
		if err != nil {
			t.Log(err)
		}
		// close channel
		close(c)
	}()
	for msg := range c {
		t.Log(msg)
	}
}
