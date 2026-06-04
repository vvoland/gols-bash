// SPDX-License-Identifier: GPL-3.0-only

package analyser

import "mvdan.cc/sh/v3/syntax"

type DeclarationKind int

const (
	// DeclarationFunction is a shell function declaration.
	DeclarationFunction DeclarationKind = iota
	// DeclarationVariable is a shell variable assignment.
	DeclarationVariable
)

// Declaration is a function or variable definition found in a parsed file.
type Declaration struct {
	Name string
	Kind DeclarationKind
	Pos  syntax.Pos
	End  syntax.Pos
}

// FindDeclarations returns function declarations and variable assignments.
func FindDeclarations(file *syntax.File) []Declaration {
	if file == nil {
		return nil
	}
	var out []Declaration
	syntax.Walk(file, func(n syntax.Node) bool {
		switch v := n.(type) {
		case *syntax.FuncDecl:
			if v.Name != nil {
				out = append(out, Declaration{
					Name: v.Name.Value,
					Kind: DeclarationFunction,
					Pos:  v.Name.Pos(),
					End:  v.Name.End(),
				})
			}
		case *syntax.Assign:
			if v.Name != nil {
				out = append(out, Declaration{
					Name: v.Name.Value,
					Kind: DeclarationVariable,
					Pos:  v.Name.Pos(),
					End:  v.Name.End(),
				})
			}
		}
		return true
	})
	return out
}
