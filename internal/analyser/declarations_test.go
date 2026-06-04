// SPDX-License-Identifier: GPL-3.0-only

package analyser

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"mvdan.cc/sh/v3/syntax"
)

func TestFindDeclarationsListsFunctionsAndVariables(t *testing.T) {
	file, err := syntax.NewParser().Parse(strings.NewReader("name=world\ngreet() {\n  local msg=hi\n}\n"), "test.sh")
	assert.NilError(t, err)

	decls := FindDeclarations(file)
	assert.Assert(t, cmp.Len(decls, 3))
	assert.Equal(t, decls[0].Name, "name")
	assert.Equal(t, decls[0].Kind, DeclarationVariable)
	assert.Equal(t, decls[1].Name, "greet")
	assert.Equal(t, decls[1].Kind, DeclarationFunction)
	assert.Equal(t, decls[2].Name, "msg")
	assert.Equal(t, decls[2].Kind, DeclarationVariable)
}
