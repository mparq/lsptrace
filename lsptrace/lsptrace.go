package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	HELP_MESSAGE = `
Usage:
  $ ./lsp-trace [command] [...command-args]

  $ ./lsp-trace -h      Display this help message.
`
)

func checkError(err error) {
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" {
		log.Fatal(HELP_MESSAGE)
	}

	tmpSockDir := filepath.Join(os.TempDir(), "lsp-trace-proxy", "sock")

	// TODO: debug file should be parameterized.
	debugF, err := os.OpenFile("/Users/mparq/code/lsp-trace-proxy/debug.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	checkError(err)
	defer debugF.Close()
	log.SetOutput(debugF)

	cmd := os.Args[1]
	execCmd := exec.Command(cmd, os.Args[2:]...)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(1)
	}()
	// Create stdout, stderr streams of type io.Reader
	//stdin, err := execCmd.StdinPipe()
	//checkError(err)
	stdout, err := execCmd.StdoutPipe()
	checkError(err)
	//stderr, err := execCmd.StderrPipe()
	//checkError(err)

	// Start command
	err = execCmd.Start()
	checkError(err)

	log.Printf("debug log opened...\n")

	f, err := os.OpenFile("/Users/mparq/code/lsp-trace-proxy/out.lsptrace", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	checkError(err)
	defer f.Close()

	// initially sent from roslyn server via stdout to be read by client
	// {"pipeName":"/var/folders/hs/1q81fggs11x5rn03t860n0n40000gn/T/713a9e7b.sock"}
	pipeName, err := PollForInitialPipeMsg(stdout)
	checkError(err)
	log.Printf("Found initial pipeName: %s\n", pipeName)
	interceptPipeName := filepath.Join(tmpSockDir, filepath.Base(pipeName))

	// create a new pipe and listen on it
	l, err := net.Listen("unix", interceptPipeName)
	checkError(err)
	defer l.Close()
	log.Printf("Created intercept pipe name: %s\n", interceptPipeName)
	log.Printf("Created intercept pipe listener\n")

	// pass new pipe to language client for it to connect to
	interceptPipeMsg := PipeMsg{
		PipeName: interceptPipeName,
	}
	interceptJson, err := json.Marshal(interceptPipeMsg)
	checkError(err)
	log.Printf("Sending intercept pipe message to original client\n")
	log.Printf("%s\n", interceptJson)
	os.Stdout.Write(interceptJson)
	os.Stdout.WriteString("\n")

	// after client reads pipeName message it should connect on the corresponding pipe
	log.Println("Listening for connections on intercept pipe...")
	conn, err := l.Accept()
	checkError(err)
	log.Println("Accepted connection on intercept pipe.")

	// connect to the original pipe which the language server broadcast that it is listening on
	log.Println("Connecting to original pipe...")
	serv_conn, err := net.Dial("unix", pipeName)
	checkError(err)
	defer serv_conn.Close()
	log.Println("Connected to original pipe.")

	in := NewPrefixWriter("->")
	out := NewPrefixWriter("<-")
	// errbuf := NewPrefixWriter("!<-", f)
	// Non-blockingly echo command output to terminal
	// go copyWith2Dest(stdin, f, os.Stdin)
	go io.Copy(io.MultiWriter(serv_conn, in), conn)
	//go copyWith2Dest(os.Stdout, f, stdout)
	go io.Copy(io.MultiWriter(conn, out), serv_conn)
	// go copyWith2Dest(os.Stderr, f, stderr)
	// go io.Copy(io.MultiWriter(os.Stderr, errbuf), stderr)

	// go io.Copy(f, in)
	// go io.Copy(f, out)

	execCmd.Wait()
}

func OpenPipeFileHandles(pipeName string) (*os.File, *os.File, error) {
	log.Println("Opening write file...")
	wf, err := os.OpenFile(pipeName, os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0777)
	if err != nil {
		return nil, nil, err
	}
	log.Println("Successfully opened write file... Opening read file...")
	f, err := os.OpenFile(pipeName, os.O_CREATE|os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, nil, err
	}
	log.Println("Successfully opened read file.")
	return f, wf, nil
}

func PollForInitialPipeMsg(pipeSender io.Reader) (string, error) {
	outScanner := bufio.NewScanner(pipeSender)

	var pipeMsg PipeMsg
	var err error
	for {
		outScanner.Scan()
		err = outScanner.Err()
		if err != nil {
			return "", err
		}
		bytes := outScanner.Bytes()
		err = json.Unmarshal(bytes, &pipeMsg)
		if err == nil && len(pipeMsg.PipeName) > 0 {
			break
		} else {
			// ignore
		}
	}
	// TODO: actually handle error
	return pipeMsg.PipeName, err

}

type PipeMsg struct {
	PipeName string `json:"pipeName"`
}

type PrefixWriter struct {
	buf    []byte
	prefix string
}

func NewPrefixWriter(prefix string) *PrefixWriter {
	return &PrefixWriter{buf: make([]byte, 0), prefix: prefix}
}

func (w *PrefixWriter) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		// Prepend '<-' to the prefix writer
		w.buf = append(w.buf, w.prefix...)

		// Append the input bytes to the buffer
		w.buf = append(w.buf, p...)

		log.Printf("%s %s\n", w.prefix, w.buf)
	}

	return len(p), nil
}

func (w *PrefixWriter) Read(p []byte) (n int, err error) {
	if n := copy(p, w.buf); n > 0 {
		w.buf = w.buf[n:]
	}
	log.Printf("%s %d bytes read\n", w.prefix, n)
	return n, nil

}