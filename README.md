# gols-bash

A Bash language server written in Go. Speaks LSP 3.17 over stdio.

## Build

```sh
./hack/binary.sh        # → _build/gols-bash
```

Requires Go 1.25+. The binary is CGO-free and statically linkable.

## Run

```sh
gols-bash                          # log to stderr at info level
gols-bash -log-level debug         # debug-level logging
gols-bash -log-file /tmp/gols.log  # redirect logs to a file
gols-bash -version                 # print version and exit
```

The server is launched by your editor's LSP client; it talks JSON-RPC
over stdin/stdout and is not meant to be invoked interactively.

## Status

Implemented features include document open, change, save, and close; Bash
parsing; ShellCheck diagnostics and fixes; formatting; navigation,
refactoring, completion, and symbols; workspace scanning and watched-file
updates; configuration; lifecycle handling; and stdio transport.
