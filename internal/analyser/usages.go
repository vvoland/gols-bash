// SPDX-License-Identifier: GPL-3.0-only

// Package analyser walks parsed bash files to find declarations and
// usages. Bash has no real scoping for top-level names, so matching is
// by string equality — same as upstream bash-language-server.
package analyser

import "mvdan.cc/sh/v3/syntax"

type UsageKind int

const (
	UsageRead UsageKind = iota
	UsageWrite
)

// Usage is one place where name appears in source — either a write
// (function decl, variable assignment) or a read (call, expansion).
type Usage struct {
	Name string
	Kind UsageKind
	Pos  syntax.Pos
	End  syntax.Pos
}

// FindUsages returns every read/write of name in file. Reads cover bare
// command invocations and ${name} / $name expansions; writes cover
// function declarations and assignments.
func FindUsages(file *syntax.File, name string) []Usage {
	if file == nil || name == "" {
		return nil
	}
	var out []Usage
	add := func(kind UsageKind, lit *syntax.Lit) {
		out = append(out, Usage{Name: name, Kind: kind, Pos: lit.Pos(), End: lit.End()})
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		switch v := n.(type) {
		case *syntax.FuncDecl:
			if v.Name != nil && v.Name.Value == name {
				add(UsageWrite, v.Name)
			}
		case *syntax.Assign:
			if v.Name != nil && v.Name.Value == name {
				add(UsageWrite, v.Name)
			}
		case *syntax.CallExpr:
			if len(v.Args) == 0 || len(v.Args[0].Parts) != 1 {
				return true
			}
			lit, ok := v.Args[0].Parts[0].(*syntax.Lit)
			if !ok || lit.Value != name {
				return true
			}
			add(UsageRead, lit)
		case *syntax.ParamExp:
			if v.Param != nil && v.Param.Value == name {
				add(UsageRead, v.Param)
			}
		}
		return true
	})
	return out
}
