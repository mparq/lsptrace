package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"lsptrace/internal"
	"lsptrace/internal/pipeline"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const (
	HELP_MESSAGE = `
Usage:
  $ ./lsptrace [command] [...command-args]

  $ ./lsptrace -h      Display this help message.
`
)

var (
	// Output file which the program will write lsp traces to
	// while processing lsp communication
	TRACE_OUTPUT = os.Getenv("LSPTRACE_TRACE_OUTPUT")
	// Output file for debug logs. Due to the nature of the program
	// stdout is not usable for logging.
	DEBUG_OUTPUT = os.Getenv("LSPTRACE_DEBUG_OUTPUT")
	// Command to run the language server e.g. `dotnet <roslyndllpath>``.
	// If this is not set, the program will assume its first argument is the
	// command to run. If the cmd is space-separated then it will be split
	// and the first part will be used as command in exec.Command and the
	// other parts will be pre-pended to the args passed to lsptrace
	// IMPORTANT: If LSPTRACE_LANGUAGE_SERVER_CMD is set then lsptrace will
	// ignore parsing command line flags, because the caller of the command
	// may expect to pass flags directly to the exe. in this case, all
	// lsptrace configuration should be set through environment vars
	LANGUAGE_SERVER_CMD = os.Getenv("LSPTRACE_LANGUAGE_SERVER_CMD")
	// '1' means that the lsp communication will start with named pipe negotation
	// meaning that the server will create a named pipe and then pass a single
	// json message with 'pipeName' over stdout which the client should listen for
	// and then connect to - from that point all communication will go through the
	// pipe instead of stdin/stdout
	HANDLE_NAMED_PIPES, _ = strconv.ParseBool(os.Getenv("LSPTRACE_HANDLE_NAMED_PIPES"))
	CLI_ARGS              = os.Args[1:]
)

func checkError(err error) {
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
}

func setupLogger(filePath string) (func(), error) {
	// TODO: debug file should be parameterized.
	debugF, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	log.SetOutput(debugF)
	log.Printf("setup logger to write to: %s\n", filePath)
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

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" {
		log.Fatal(HELP_MESSAGE)
	}

	// configuration
	flag.StringVar(&DEBUG_OUTPUT, "debug_output", DEBUG_OUTPUT, "filepath to write debug logs to.")
	flag.StringVar(&TRACE_OUTPUT, "trace_output", TRACE_OUTPUT, "filepath to write lsp traces to.")
	flag.BoolVar(&HANDLE_NAMED_PIPES, "handle_named_pipes", HANDLE_NAMED_PIPES, "whether lsp communication will use named pipes. if true, lsptrace will expect an initial named pipe handshake.")

	if len(LANGUAGE_SERVER_CMD) <= 0 {
		// only parse flags from command line if LANGUAGE_SERVER_CMD isn't explicitly set
		flag.Parse()
		CLI_ARGS = flag.Args()
	}

	if len(TRACE_OUTPUT) < 1 {
		log.Fatalf("LSPTRACE_TRACE_OUTPUT or --trace_output must be set\n")
	}

	// setup resources

	// setup tmp dir
	// TODO: temporary sockets should be removed after
	// the log file probably shouldn't be removed.
	tmpDir, err := os.MkdirTemp("", "lsp-trace-proxy")
	checkError(err)

	// setup logger
	debugPath := filepath.Join(tmpDir, "debug.log")
	if len(DEBUG_OUTPUT) > 0 {
		debugPath, err = resolveLocalPath(DEBUG_OUTPUT)
		checkError(err)
	}
	logCloser, err := setupLogger(debugPath)
	checkError(err)
	defer logCloser()

	// open trace file
	tracePath, err := resolveLocalPath(TRACE_OUTPUT)
	checkError(err)
	traceOut, err := os.OpenFile(tracePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		err = errors.Join(errors.New("error opening trace output file"), err)
		checkError(err)
	}
	defer traceOut.Close()

	// TODO: handle interrupts properly and cleanup
	handleInterrupt(nil)

	log.Printf("debug log opened...\n")

	// setup command
	execCmd := setupLanguageServerCommand()
	log.Printf("execCmd created.: %s\n", execCmd.String())

	pipes, err := createLspPipes(execCmd, tmpDir, HANDLE_NAMED_PIPES)
	checkError(err)
	defer pipes.Close()

	cIn, cOut, sIn, sOut := pipes.CIn(), pipes.COut(), pipes.SIn(), pipes.SOut()
	reqMap := internal.NewRequestMap()
	lspTracer := internal.NewLSPTracer(reqMap)
	clientPipeline := pipeline.NewPipeline(cOut, sIn, traceOut, lspTracer, "client")
	serverPipeline := pipeline.NewPipeline(sOut, cIn, traceOut, lspTracer, "server")
	// TODO: handle closing
	_ = clientPipeline.Run()
	_ = serverPipeline.Run()

	execCmd.Wait()
}

func createLspPipes(execCmd *exec.Cmd, tmpDir string, handleNamedPipes bool) (lspPipe LSPPipe, err error) {
	if handleNamedPipes {
		lspPipe = NewNamedPipesLSPPipe(execCmd, tmpDir)
	} else {
		lspPipe = NewStdInOutLSPPipe(execCmd)
	}
	err = lspPipe.Setup()
	if err != nil {
		err = errors.Join(errors.New("could not set up named pipes lsp communication"), err)
		return nil, err
	}
	return lspPipe, err

}

type LSPPipe interface {
	Setup() error
	CIn() io.Writer
	COut() io.Reader
	SIn() io.Writer
	SOut() io.Reader
	Close()
}

type StdInOutLSPPipe struct {
	execCmd *exec.Cmd
	sIn     io.WriteCloser
	sOut    io.ReadCloser
}

func NewStdInOutLSPPipe(execCmd *exec.Cmd) *StdInOutLSPPipe {
	return &StdInOutLSPPipe{execCmd: execCmd}
}

func (p *StdInOutLSPPipe) Setup() error {
	sIn, err := p.execCmd.StdinPipe()
	if err != nil {
		return err
	}
	p.sIn = sIn
	sOut, err := p.execCmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.sOut = sOut
	err = p.execCmd.Start()
	if err != nil {
		err = errors.Join(errors.New("error starting lsp command"), err)
		return err
	}
	return nil
}

// note that "client in" pipe is the stdout of this program
// and vice-versa the "client out" pipe is the stdin of this program
func (p *StdInOutLSPPipe) CIn() io.Writer {
	return os.Stdout
}

func (p *StdInOutLSPPipe) COut() io.Reader {
	return os.Stdin
}

func (p *StdInOutLSPPipe) SIn() io.Writer {
	return p.sIn
}

func (p *StdInOutLSPPipe) SOut() io.Reader {
	return p.sOut
}

func (p *StdInOutLSPPipe) Close() {
	p.sIn.Close()
	p.sOut.Close()
}

type NamedPipesLSPPipe struct {
	execCmd           *exec.Cmd
	pipeDir           string
	interceptListener net.Listener
	clientConnection  net.Conn
	serverConnection  net.Conn
}

func NewNamedPipesLSPPipe(execCmd *exec.Cmd, pipeDir string) *NamedPipesLSPPipe {
	return &NamedPipesLSPPipe{execCmd: execCmd, pipeDir: pipeDir}
}

func (p *NamedPipesLSPPipe) Setup() error {
	// in named pipe flow, expectation is that we start server
	// the server will respond with a json message over stdout {"pipeName": "..."}
	// the client should internal.this message and then connect to the named pipe in the message
	// from that point, all client/server lsp comms go through the pipe

	stdout, err := p.execCmd.StdoutPipe()
	if err != nil {
		err = errors.Join(errors.New("could not get stdout pipe of lsp command"), err)
		return err
	}
	// Start command
	err = p.execCmd.Start()
	if err != nil {
		err = errors.Join(errors.New("could not start lsp command"), err)
		return err
	}
	// initially sent from roslyn server via stdout to be read by client
	// {"pipeName":"/var/folders/hs/1q81fggs11x5rn03t860n0n40000gn/T/713a9e7b.sock"}
	pipeName, err := pollForInitialPipeMsg(stdout)
	if err != nil {
		err = errors.Join(errors.New("error polling for initial pipe message"), err)
		return err
	}
	log.Printf("Found initial pipeName: %s\n", pipeName)
	// create a new intercept pipe which will be sent to client to connect to instead
	interceptPipeName := filepath.Join(p.pipeDir, filepath.Base(pipeName))
	l, err := net.Listen("unix", interceptPipeName)
	p.interceptListener = l
	if err != nil {
		err = errors.Join(errors.New("could not setup intercept pipe"), err)
		return err
	}
	log.Printf("Created intercept pipe name: %s\n", interceptPipeName)
	log.Printf("Created intercept pipe listener\n")
	interceptPipeMsg := PipeMsg{
		PipeName: interceptPipeName,
	}
	interceptJson, err := json.Marshal(interceptPipeMsg)
	if err != nil {
		err = errors.Join(errors.New("could not create intercept pipe msg json"), err)
		return err
	}
	log.Printf("Sending intercept pipe message to original client\n")
	log.Printf("%s\n", interceptJson)
	os.Stdout.Write(interceptJson)
	os.Stdout.WriteString("\n")

	// after client reads pipeName message it should connect on the corresponding pipe
	// TODO: add timeout
	log.Println("Listening for connections on intercept pipe...")
	conn, err := l.Accept()
	p.clientConnection = conn
	if err != nil {
		err = errors.Join(errors.New("error listening for connection from client on created intercept pipe"), err)
		return err
	}
	log.Println("Accepted connection on intercept pipe.")

	// connect to the original pipe which the language server broadcast that it is listening on
	log.Println("Connecting to original pipe...")
	serv_conn, err := net.Dial("unix", pipeName)
	p.serverConnection = serv_conn
	if err != nil {
		err = errors.Join(errors.New("unable to connect to original pipe given from server"), err)
		return err
	}
	log.Println("Connected to original pipe.")
	return nil
}

func (p *NamedPipesLSPPipe) CIn() io.Writer {
	return p.clientConnection
}

func (p *NamedPipesLSPPipe) COut() io.Reader {
	return p.clientConnection
}

func (p *NamedPipesLSPPipe) SIn() io.Writer {
	return p.serverConnection
}

func (p *NamedPipesLSPPipe) SOut() io.Reader {
	return p.serverConnection
}

func (p *NamedPipesLSPPipe) Close() {
	p.serverConnection.Close()
	p.clientConnection.Close()
	p.interceptListener.Close()
}

func pollForInitialPipeMsg(pipeSender io.Reader) (string, error) {
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

func setupLanguageServerCommand() *exec.Cmd {
	var cmd string
	var args []string
	if LANGUAGE_SERVER_CMD == "" {
		log.Println("language server command not specified. assumed to be first argument.")
		cmd = CLI_ARGS[0]
		args = CLI_ARGS[1:]
	} else {
		// for roslyn we should configure LSPTRACE_LANGUAGE_SERVER_CMD = "dotnet <path-to-roslyn-dll>"
		// when running vscode
		log.Printf("language server command specified. lsptrace will run %s with given args\n", LANGUAGE_SERVER_CMD)
		cmdParts := strings.Split(LANGUAGE_SERVER_CMD, " ")
		cmd = cmdParts[0]
		if len(cmdParts) > 1 {
			args = append(cmdParts[1:], CLI_ARGS...)
		} else {
			args = CLI_ARGS
		}
	}
	execCmd := exec.Command(cmd, args...)
	return execCmd
}

func resolveLocalPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			err = errors.Join(errors.New("error resolving user home path"), err)
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}
	return path, nil
}
