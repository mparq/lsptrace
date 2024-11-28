package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	REQUEST      = "request"
	NOTIFICATION = "notification"
	RESPONSE     = "response"
	ERROR        = "error"
)

// Represents the raw jsonrpc message sent b/w client and server
// as part of the LSP.
type RawLSPMessage struct {
	JsonRpc string  `json:"jsonrpc"`
	Id      *int64  `json:"id,omitempty"`
	Method  *string `json:"method,omitempty"`
	// NOTE: json.RawMessage used here to differentiate between null value and empty
	// if field is empty, then json.RawMessage will be nil. If field is json null then
	// the RawMessage will be the string "null"
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

func MessageKind(lspMessage *RawLSPMessage) string {
	switch {
	case lspMessage.Id != nil && lspMessage.Method != nil:
		return "request"
	case lspMessage.Id != nil && lspMessage.Method == nil && lspMessage.Result != nil:
		return "response"
	case lspMessage.Id != nil && lspMessage.Method == nil && lspMessage.Error != nil:
		return "error"
	case lspMessage.Id == nil && lspMessage.Method != nil:
		return "notification"
	}
	return "unknown"
}

type LSPTrace struct {
	// LSP message kind: 'request' | 'response' | 'error' | 'notification'
	MessageKind string `json:"msgKind"`
	// Where the message was sent from 'client' | 'server'
	SentFrom string `json:"from"`
	// The LSP method. Empty for notifications, will be looked up for lsp responses
	Method *string `json:"method,omitempty"`
	Id     *int64  `json:"id,omitempty"`
	// UTC timestamp the message was received by the tracer
	Timestamp time.Time `json:"timestamp"`
	// The parsed raw json message ('params' and 'result' will be here)
	Message RawLSPMessage `json:"msg"`
}

// Convert Raw LSP JSON body into LSPTrace.
// Will modify t in place.
func (t *LSPTrace) FromRaw(rawLSPMessage *RawLSPMessage, sentFrom string) {
	messageKind := MessageKind(rawLSPMessage)
	*t = LSPTrace{
		MessageKind: messageKind,
		Method:      rawLSPMessage.Method,
		Id:          rawLSPMessage.Id,
		Message:     *rawLSPMessage,
		SentFrom:    sentFrom,
		Timestamp:   time.Now().UTC(),
	}
}

func (m RawLSPMessage) String() string {
	fields := make([]string, 0)
	if m.Method != nil {
		fields = append(fields, fmt.Sprintf("Method=%s", *m.Method))
	}
	if m.Id != nil {
		fields = append(fields, fmt.Sprintf("Id=%v", *m.Id))
	}
	if m.Params != nil {
		fields = append(fields, fmt.Sprintf("Params=%v", string(m.Params)))
	}
	if m.Result != nil {
		fields = append(fields, fmt.Sprintf("Result=%v", string(m.Result)))
	}
	if m.Error != nil {
		fields = append(fields, fmt.Sprintf("Error=%v", string(m.Error)))
	}
	return fmt.Sprintf("RawLSPMessage[%s]", strings.Join(fields[0:], "|"))
}

func (t LSPTrace) String() string {
	fields := make([]string, 0)
	df := func(fn string, val any) string {
		return fmt.Sprintf("%s=%v", fn, val)
	}
	fields = append(fields, df("MessageKind", t.MessageKind), df("SentFrom", t.SentFrom))
	if t.Method != nil {
		fields = append(fields, df("Method", *t.Method))
	}
	if t.Id != nil {
		fields = append(fields, df("Id", *t.Id))
	}
	fields = append(fields, df("Message", t.Message))
	fields = append(fields, df("Timestamp", t.Timestamp))

	return fmt.Sprintf("LSPTrace[%s]", strings.Join(fields, "|"))
}
