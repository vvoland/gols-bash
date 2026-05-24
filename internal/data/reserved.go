// SPDX-License-Identifier: GPL-3.0-only

package data

// ReservedWords is the bash reserved-word list.
// See https://www.gnu.org/software/bash/manual/html_node/Reserved-Word-Index.html
var ReservedWords = []string{
	"!", "[[", "]]", "{", "}",
	"case", "do", "done",
	"elif", "else", "esac",
	"fi", "for", "function",
	"if", "in",
	"select",
	"then", "time",
	"until",
	"while",
}

var reservedSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(ReservedWords))
	for _, w := range ReservedWords {
		m[w] = struct{}{}
	}
	return m
}()

func IsReservedWord(word string) bool {
	_, ok := reservedSet[word]
	return ok
}
