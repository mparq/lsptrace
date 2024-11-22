# LSP Trace

Easy (hopefully) tracing of LSP communication for diffing and debugging purposes.

- tracer: `lsptrace <language-server-exe>` in place of `<language-server-exe>` to collect lsptrace logs
- viewer: load lsptrace log file and quickly search/filter and have ui which helps grok the flow of lsp communication in the trace.

## Motivation

Wanting to have an easy binary to plug in front of a language server to intercept LSP communication between language client and server and log traces in a standard format for later parsing and friendlier UI. For the purposes of both debugging LSP flow within a client implementation and also diffing between flows in different clients (e.g. nvim vs. vscode).

Prior work and inspiration is [vscode-lsp-inspector](https://github.com/Microsoft/language-server-protocol-inspector).

Mainly, I got annoyed having to either monkey patch things or enable the "debug" log level -> parse through the logs from within nvim or vscode -> select out the rpc events I'm looking for -> figure out which rpc events were fired from which language server (important for razor and roslyn which have complicated back-and-forth flows)

## lsptrace format

Could consider json, like in the original language-server-protocol-inspector, but I'd like to have a simpler format which is quickly grokkable and diffable without the UI if possible. There are a few fields which I want added to each jsonrpc event: timestamp, serverName, msgKind (client_request, client_response, serv_request, serv_response, client_notification, server_notification), method, rawEvent

Maybe pipe-delimited would work, also ensure rawEvent has no new lines so that each line of file corresponds to one event.

```
timestamp|method|msgKind|serverName|rawEvent
```

