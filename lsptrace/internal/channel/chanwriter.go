package channel

type Writer struct {
	out chan []byte
}

func NewWriter() *Writer {
	return &Writer{}
}

func (r *Writer) Out(out chan []byte) {
	r.out = out
}

func (r *Writer) Write(p []byte) (n int, err error) {
	if r.out == nil {
		panic("chanio.writer: Out channel must be set before calling Write")
	}
	// TODO: need to handle error?
	r.out <- p
	return n, err
}
