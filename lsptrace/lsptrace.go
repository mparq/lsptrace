package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"lsptrace/internal"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
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

func setupLogger() (func(), error) {
	// TODO: debug file should be parameterized.
	debugF, err := os.OpenFile("/Users/mparq/code/lsp-trace-proxy/debug.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	log.SetOutput(debugF)
	return func() {
		debugF.Close()
	}, err
}

func handleInterrupt(cleanup func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if cleanup != nil {
			cleanup()
		}
		os.Exit(1)
	}()
}

// check if args suggest that command is being run from vscode
// HACK: this is for now checking just on --telemetryLevel
// which is passed by vscode-sharp when running roslyn language server
func isVsCodeCommand(args []string) {
	for _, arg := range args {
		if strings.Contains(arg, "--telemetryLevel") {
		}
	}
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" {
		log.Fatal(HELP_MESSAGE)
	}

	roslynDll := ""
	if o, found := os.LookupEnv("LSPTRACE_ROSLYN_LS_DLL"); found {
		roslynDll = o
	}

	logCloser, err := setupLogger()
	defer logCloser()
	// TODO: handle interrupts properly and cleanup
	handleInterrupt(nil)

	log.Printf("debug log opened...\n")

	// cmd := os.Args[1]
	cmd := "dotnet"
	roslynDll = filepath.Join(
		"/Users/mparq/code",
		"ext",
		"roslyn",
		"artifacts",
		"bin",
		"Microsoft.CodeAnalysis.LanguageServer",
		"Debug",
		"net8.0",
		"Microsoft.CodeAnalysis.LanguageServer.dll",
	)

	// for vscode, this binary is expected to run the language server and we are just passed the args
	// as opposed to the 1st arg being the ls command to run
	// args := append([]string{roslynDll}, os.Args[2:]...)
	args := append([]string{roslynDll}, os.Args[1:]...)
	log.Printf(strings.Join(args, " ") + "\n")
	execCmd := exec.Command(cmd, args...)

	log.Printf("execCmd created.: %s\n", execCmd.String())

	// in named pipe flow, expectation is that we start server
	// the server will respond with a json message over stdout {"pipeName": "..."}
	// the client should internal.this message and then connect to the named pipe in the message
	// from that point, all client/server lsp comms go through the pipe

	stdout, err := execCmd.StdoutPipe()
	checkError(err)

	// Start command
	err = execCmd.Start()
	checkError(err)

	f, err := os.OpenFile("/Users/mparq/code/lsp-trace-proxy/out.lsptrace", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	checkError(err)
	defer f.Close()

	// initially sent from roslyn server via stdout to be read by client
	// {"pipeName":"/var/folders/hs/1q81fggs11x5rn03t860n0n40000gn/T/713a9e7b.sock"}
	pipeName, err := PollForInitialPipeMsg(stdout)
	checkError(err)
	log.Printf("Found initial pipeName: %s\n", pipeName)

	// create a new intercept pipe which will be sent to client to connect to instead
	tmpDir, err := os.MkdirTemp("", "lsp-trace-proxy")
	checkError(err)
	log.Printf("Created tmp dir for intercept socket: %s", tmpDir)
	interceptPipeName := filepath.Join(tmpDir, filepath.Base(pipeName))
	l, err := net.Listen("unix", interceptPipeName)
	checkError(err)
	defer func() {
		l.Close()
		os.RemoveAll(tmpDir)
	}()
	log.Printf("Created intercept pipe name: %s\n", interceptPipeName)
	log.Printf("Created intercept pipe listener\n")
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

	c := make(chan *internal.RawLSPMessage)
	sc := make(chan *internal.RawLSPMessage)
	tc := make(chan internal.LSPTrace, 5)
	stc := make(chan internal.LSPTrace, 5)

	clientIntercept := internal.NewInterceptor(c)
	serverIntercept := internal.NewInterceptor(sc)
	_ = clientIntercept // remove
	_ = serverIntercept // remove

	reqMap := internal.NewRequestMap()
	clientTracer := internal.NewLSPTracer("client", reqMap)
	serverTracer := internal.NewLSPTracer("server", reqMap)
	clientTracer.In(c)
	clientTracer.Out(tc)
	serverTracer.In(sc)
	serverTracer.Out(stc)

	go clientTracer.Run()
	go serverTracer.Run()

	go func() {
		for trace := range tc {
			log.Println(trace)
		}
	}()
	go func() {
		for trace := range stc {
			log.Println(trace)
		}
	}()

	// errbuf := NewPrefixWriter("!<-", f)
	// Non-blockingly echo command output to terminal
	// go copyWith2Dest(stdin, f, os.Stdin)
	// NOTE: can write raw json rpc to log by using log.Writer() in io.MultiWriter args
	go io.Copy(io.MultiWriter(serv_conn, clientIntercept), conn)
	//go copyWith2Dest(os.Stdout, f, stdout)
	go io.Copy(io.MultiWriter(conn, serverIntercept), serv_conn)
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
