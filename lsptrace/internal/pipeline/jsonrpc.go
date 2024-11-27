package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"lsptrace/internal"
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

// JsonRpcStage is designed to by streamed to as an io.Writer
// It will parse the incoming stream and output raw lsp json bodies when found
// for further processing via the output channel. The caller is responsible
// for managing the channel as the interceptor can't know when to stop
type JsonRpcStage struct {
	// vars to track rpc state
	gotHeader         bool
	nextContentLength int
	scanBuf           *bytes.Buffer
}

func NewJsonRpcStage() *JsonRpcStage {
	scanBuf := new(bytes.Buffer)
	return &JsonRpcStage{scanBuf: scanBuf}
}

func (t *JsonRpcStage) Run(in chan []byte) chan *internal.RawLSPMessage {
	out := make(chan *internal.RawLSPMessage)
	go func() {
		for read := range in {
			// write to scan buffer
			t.scanBuf.Write(read)
			var err error
			var more bool
			for {
				// try to read as much as possible
				more, err = t.next(out)
				if err != nil || !more {
					break
				}
			}
			if err != nil {
				log.Printf("interceptor:run Error parsing next %s \n", err)
			}
		}
		// close out channel after in closes
		close(out)
	}()
	return out
}

// next advances in the rpc protocol read state
// will return true if something was done (implies
// that the caller should keep calling in case there
// is more to do). returns false if nothing could be done
func (t *JsonRpcStage) next(out chan *internal.RawLSPMessage) (bool, error) {
	switch {
	case !t.gotHeader:
		nl := bytes.Index(t.scanBuf.Bytes(), []byte("\r\n\r\n"))
		if nl < 0 {
			return false, nil
		}
		// read including the \r\n\r\n
		readBuf := make([]byte, nl+4)
		nr, err := t.scanBuf.Read(readBuf)
		if err != nil || nr != nl+4 {
			err = errors.Join(err, EBUFREAD)
			return false, err
		}

		// split header section excluding the \r\n\r\n
		headers := strings.Split(string(readBuf[0:nl]), "\r\n")
		contentLength := 0
		for _, header := range headers {
			parts := strings.Split(header, ": ")
			// HACK: handle garbage in front of content-length header which is seen in wild
			if strings.HasSuffix(parts[0], "Content-Length") {
				contentLength, err = strconv.Atoi(parts[1])
				if err != nil {
					break
				}
			}
		}
		if contentLength == 0 {
			err = EREADCONTENTLENGTH
			log.Printf("jsonrpc: headers: %s\n", string(readBuf[0:nl]))
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
			lspMessage := new(internal.RawLSPMessage)
			err := json.Unmarshal(readBuf, lspMessage)
			if err != nil {
				err = errors.Join(err, EPARSE)
				t.nextContentLength = 0
				t.gotHeader = false
				return false, err
			}
			out <- lspMessage
			// reset rpc read state
			t.nextContentLength = 0
			t.gotHeader = false
			return true, nil
		}
	}
	return false, nil
}
