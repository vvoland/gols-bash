// SPDX-License-Identifier: GPL-3.0-only

// Package data holds static lookups: bash builtins and reserved words.
// The lists track upstream bash-language-server.
package data

// Builtins is the canonical bash builtin set (`compgen -b` on a clean bash).
var Builtins = []string{
	".", ":", "[",
	"alias", "bg", "bind", "break", "builtin",
	"caller", "cd", "command", "compgen", "compopt", "complete", "continue",
	"declare", "dirs", "disown",
	"echo", "enable", "eval", "exec", "exit", "export",
	"false", "fc", "fg",
	"getopts",
	"hash", "help", "history",
	"jobs",
	"kill",
	"let", "local", "logout",
	"popd", "printf", "pushd", "pwd",
	"read", "readonly", "return",
	"set", "shift", "shopt", "source", "suspend",
	"test", "times", "trap", "true", "type", "typeset",
	"ulimit", "umask", "unalias", "unset",
	"wait",
}

var builtinSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(Builtins))
	for _, b := range Builtins {
		m[b] = struct{}{}
	}
	return m
}()

func IsBuiltin(word string) bool {
	_, ok := builtinSet[word]
	return ok
}
