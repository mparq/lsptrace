package channel

import (
	"bytes"
	"testing"
)

func TestChanWriter(t *testing.T) {
	// set up channel
	// send string to channel
	// assert string written out from writer is same
	test := []byte("test string")
	c := make(chan []byte)
	wr := NewWriter(c)

	go func() {
		wr.Write([]byte(test))
	}()

	out := <-c

	if !bytes.Equal(out, test) {
		t.Fatalf("channel writer should directly output input onto channel. actual: %s, expected: %s", out, test)
	}

}
