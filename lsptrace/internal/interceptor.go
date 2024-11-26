package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
)

const (
	SCANBUF_SIZE = 8 * 1024
)

var (
	EPARSE             = errors.New("interceptor: could not parse json into lsp message")
	EBUFREAD           = errors.New("interceptor: error occurred when parsing new line")
	EREADCONTENTLENGTH = errors.New("interceptor: unexpected error reading content length header")
)

// Interceptor is designed to by streamed to as an io.Writer
// It will parse the incoming stream and output raw lsp json bodies when found
// for further processing via the output channel. The caller is responsible
// for managing the channel as the interceptor can't know when to stop
type Interceptor struct {
	// vars to track rpc state
	gotHeader         bool
	nextContentLength int
	scanBuf           *bytes.Buffer
	in                chan []byte
	out               chan *RawLSPMessage
}

func NewInterceptor() *Interceptor {
	scanBuf := new(bytes.Buffer)
	return &Interceptor{scanBuf: scanBuf}
}

func (t *Interceptor) In(in chan []byte) {
	t.in = in
}

func (t *Interceptor) Out(out chan *RawLSPMessage) {
	t.out = out
}

func (t *Interceptor) Run() {
	for read := range t.in {
		t.scanBuf.Write(read)
		var err error
		for {
			more, err := t.next()
			if err != nil {
				break
			}
			if !more {
				break
			}
		}
		if err != nil {
			log.Printf("interceptor:run Error parsing next %s \n", err)
		}
	}
	// close out channel after in closes
	close(t.out)
}

// next advances in the rpc protocol read state
// will return true if something was done (implies
// that the caller should keep calling in case there
// is more to do). returns false if nothing could be done
func (t *Interceptor) next() (bool, error) {
	readBuf := make([]byte, 1024*8)

	switch {
	case !t.gotHeader:
		nl := bytes.Index(t.scanBuf.Bytes(), []byte{'\r', '\n', '\r', '\n'})
		if nl < 0 {
			return false, nil
		}
		// read including the \r\n\r\n
		readSlice := readBuf[:nl+4]
		nr, err := t.scanBuf.Read(readSlice)
		if err != nil || nr != nl+4 {
			err = errors.Join(err, EBUFREAD)
			return false, err
		}

		// split header section excluding the \r\n\r\n
		headers := strings.Split(string(readSlice[0:nl]), "\r\n")
		contentLength := 0
		for _, header := range headers {
			parts := strings.Split(header, ": ")
			if parts[0] == "Content-Length" {
				contentLength, err = strconv.Atoi(parts[1])
				if err != nil {
					break
				}
			}
		}
		if contentLength == 0 {
			err = EREADCONTENTLENGTH
			return false, err
		}

		t.nextContentLength = contentLength
		t.gotHeader = true
		return true, nil

	case t.nextContentLength > 0:
		// read content
		if t.scanBuf.Len() >= t.nextContentLength {
			readBuf := make([]byte, t.nextContentLength)
			t.scanBuf.Read(readBuf)

			// parse raw json message
			lspMessage := new(RawLSPMessage)
			err := json.Unmarshal(readBuf, lspMessage)
			if err != nil {
				err = errors.Join(err, EPARSE)
				return false, err
			}
			t.out <- lspMessage
			// reset rpc read state
			t.nextContentLength = 0
			t.gotHeader = false
			return true, nil
		}
	}
	return false, nil
}

func (t *Interceptor) Write(buf []byte) (int, error) {
	nw, err := t.scanBuf.Write(buf)

	if err != nil {
		return nw, err
	}

	for {
		more, err := t.next()
		if err != nil {
			log.Printf("Error on write: %s\n", err)
			break
		}
		if !more {
			break
		}
	}
	return nw, err
}
