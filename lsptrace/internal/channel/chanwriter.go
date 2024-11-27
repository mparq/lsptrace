package channel

type Writer struct {
	out chan []byte
}

func NewWriter(out chan []byte) *Writer {
	return &Writer{out}
}

func (r *Writer) Write(p []byte) (n int, err error) {
	if r.out == nil {
		panic("chanio.writer: Out channel must be set before calling Write")
	}
	// TODO: need to handle error?
	go func() {
		// send to channel but don't block write
		r.out <- p
	}()
	return n, err
}
