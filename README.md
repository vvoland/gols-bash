# gols-bash

A Bash language server written in Go. Speaks LSP 3.17 over stdio.

## Build

```sh
./hack/binary.sh        # → _build/gols-bash
```

Requires Go 1.25+. The binary is CGO-free and statically linkable.

## Run

```sh
gols-bash               # log to stderr
gols-bash -verbose      # debug-level logging
gols-bash -log /tmp/gols-bash.log
```

The server is launched by your editor's LSP client; it talks JSON-RPC
over stdin/stdout and is not meant to be invoked interactively.

## Status

Early scaffolding. Currently handles `initialize`, `initialized`,
`shutdown`, and `exit` only. Document sync, diagnostics, hover,
completion, and goto-definition are not yet implemented.
