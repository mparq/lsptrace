# LSP Trace

Easy tracing of LSP communication for diffing and debugging purposes.

- [x] tracer: `lsptrace <language-server-exe>` in place of `<language-server-exe>` to collect lsptrace logs
- [ ] viewer: load lsptrace log file and quickly search/filter and have ui which helps grok the flow of lsp communication in the trace.

`lsptrace` proxies the communication between client and server and parses the lsp protocol to extract lsp messages.
It adds some extra properties to each message such as whether it was sent from client/server and what kind of message it is (request|response|notification|error) so that the trace file is ready for analysis without extra parsing. A nice property of this is that the trace file is easily greppable just in a text editor (with `:set nowrap`).
It is designed to be easily pluggable in front of the server application without having to mess with the lsp client-specific configuration, so you can quickly and easily grab lsptraces from different clients.

Inspired by [vscode-lsp-inspector](https://github.com/Microsoft/language-server-protocol-inspector).

Mainly, I got annoyed having to either monkey patch things or enable the "debug" log level -> parse through the logs from within nvim or vscode -> select out the rpc events I'm looking for -> figure out which rpc events were fired from which language server (important for razor and roslyn which have complicated back-and-forth flows)

## Usage

### e.g. roslyn.nvim in neovim

```lua
require('roslyn').setup {
  exe = {
    -- use lsptrace and pass lsptrace specific flags first
    "/path/to/lsptrace/bin/lsptrace",
    "--handle_named_pipes=true",
    "--debug_output=~/.lsptrace/roslyn-nvim.debug",
    "--trace_output=~/.lsptrace/roslyn-nvim.lsptrace",
    -- original exe
    "dotnet",
    vim.fs.joinpath(
        vim.fn.stdpath 'data' --[[@as string]],
        'mason',
        'packages',
        'roslyn',
        'libexec',
        'Microsoft.CodeAnalysis.LanguageServer.dll'
    )

  },
  args = {
    '--logLevel=Information',
    '--extensionLogDirectory=' .. vim.fs.dirname(vim.lsp.get_log_path()),
    '--razorSourceGenerator=' .. vim.fs.joinpath(
      vim.fn.stdpath 'data' --[[@as string]],
      'mason',
      'packages',
      'roslyn',
      'libexec',
      'Microsoft.CodeAnalysis.Razor.Compiler.dll'
    ),
    '--razorDesignTimePath=' .. vim.fs.joinpath(
      vim.fn.stdpath 'data' --[[@as string]],
      'mason',
      'packages',
      'rzls',
      'libexec',
      'Targets',
      'Microsoft.NET.Sdk.Razor.DesignTime.targets'
    ),
  },
  config = {
    on_attach = require 'lspattach',
    capabilities = capabilities,
    handlers = require 'rzls.roslyn_handlers',
  },
}

```

### e.g. vscode-sharp in vscode

vscode sharp can be configured to point to an executable which runs the language server via `dotnet.server.path`

```json
{
    "dotnet.server.path": "/path/to/lsptrace/bin/lsptrace",
}
```

however, in this case, we can't easily modify the arguments passed to the command because the command is built up by the extension.
We need to configure lsptrace both with `LSPTRACE_LANGUAGE_SERVER_CMD` and other options via environment variables. The easiest way
to do this is to run VSCode via the command line e.g.

```sh
#!/bin/bash
set -e

LSPTRACE_TRACE_OUTPUT=~/.lsptrace/out.lsptrace \
LSPTRACE_DEBUG_OUTPUT=~/.lsptrace/debug.out \
LSPTRACE_LANGUAGE_SERVER_CMD="dotnet ~/code/ext/roslyn/artifacts/bin/Microsoft.CodeAnalysis.LanguageServer/Debug/net8.0/Microsoft.CodeAnalysis.LanguageServer.dll" \
LSPTRACE_HANDLE_NAMED_PIPES=true \
code-insiders .
```

Note that `LSPTRACE_LANGUAGE_SERVER_CMD` is `dotnet <path-to-roslyn-dll>` and `LSPTRACE_HANDLE_NAMED_PIPES` is set because of the special name pipe initialization that the roslyn language server requires.

## Build lsptrace from source

- `go` is required.
- Run `./build.sh` which will output the binary -> `bin/lsptrace`

## Output

### lsptrace format

lsptrace will parse lsp communication and output a json per message which follows the following format:

```go
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
```

It's slightly different but can easily be converted into the format expected by `language-server-protocol-inspector`.




