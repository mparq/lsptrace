package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mparq/lsptrace/internal"
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

// JsonRpcStage is designed to by streamed to as an io.Writer
// It will parse the incoming stream and output raw lsp json bodies when found
// for further processing via the output channel. The caller is responsible
// for managing the channel as the interceptor can't know when to stop
type JsonRpcStage struct {
	// vars to track rpc state
	gotHeader         bool
	nextContentLength int
	scanBuf           *bytes.Buffer

	readingChunk bool
}

func NewJsonRpcStage() *JsonRpcStage {
	scanBuf := new(bytes.Buffer)
	return &JsonRpcStage{scanBuf: scanBuf}
}

func (t *JsonRpcStage) Run(in chan []byte) chan *internal.RawLSPMessage {
	out := make(chan *internal.RawLSPMessage)
	go func() {
		for read := range in {
			if t.readingChunk {
				panic("unexpected: reading chunk in parallel")
			}
			t.readingChunk = true
			log.Printf("pre scanbuf write: jsonrpc input: %s\n", string(read))
			// write to scan buffer
			t.scanBuf.Write(read)
			var err error
			var more bool
			for {
				log.Printf("post scanbuf write: attempt next")
				// try to read as much as possible
				more, err = t.next(out)
				if err != nil || !more {
					break
				}
			}
			if err != nil {
				log.Printf("interceptor:run Error parsing next %s \n", err)
			}
			t.readingChunk = false
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
		log.Printf("found header: %s\n", string(t.scanBuf.Bytes()[0:nl]))
		// read including the last \r\n\r\n (nl+4)
		readBuf := make([]byte, nl+4)
		nr, err := t.scanBuf.Read(readBuf)
		if err != nil || nr != nl+4 {
			err = errors.Join(err, EBUFREAD)
			return false, err
		}

		// split header section by \r\n excluding the last \r\n\r\n (nl)
		headers := strings.Split(string(readBuf[0:nl]), "\r\n")
		contentLength := 0
		for _, header := range headers {
			// HACK: handle garbage in front of content-length header which is seen in wild
			clIndex := strings.Index(header, "Content-Length: ")
			if clIndex >= 0 {
				clValue := header[clIndex+len("Content-Length: "):]
				contentLength, err = strconv.Atoi(clValue)
				if err != nil {
					log.Printf("error converting content-length value [%s]: %s", clValue, err)
					break
				}
			}
		}
		if contentLength == 0 {
			err = errors.Join(err, EREADCONTENTLENGTH)
			panic(fmt.Sprintf("err:%s === headers:%s", err, string(readBuf[0:nl])))
			// return false, err
		}

		t.nextContentLength = contentLength
		t.gotHeader = true
		return true, nil

	case t.nextContentLength > 0:
		// read content
		log.Printf("scanBuf length: %v\n", t.scanBuf.Len())
		if t.scanBuf.Len() >= t.nextContentLength {
			readBuf := make([]byte, t.nextContentLength)
			t.scanBuf.Read(readBuf)

			// parse raw json message
			lspMessage := new(internal.RawLSPMessage)
			err := json.Unmarshal(readBuf, lspMessage)
			if err != nil {
				log.Printf("unmarshall: err on %s", string(readBuf))
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
