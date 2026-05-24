// SPDX-License-Identifier: GPL-3.0-only

package lsp

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestOffsetForPositionASCII(t *testing.T) {
	src := "echo hi\nworld\n"
	assert.Equal(t, OffsetForPosition(src, 0, 0), 0)
	assert.Equal(t, OffsetForPosition(src, 0, 4), 4)
	assert.Equal(t, OffsetForPosition(src, 1, 0), 8)
	assert.Equal(t, OffsetForPosition(src, 1, 5), 13)
}

func TestOffsetForPositionUTF16Surrogates(t *testing.T) {
	// "𐐷" = U+10437 = 4 UTF-8 bytes, 2 UTF-16 code units.
	src := "a𐐷b"
	assert.Equal(t, OffsetForPosition(src, 0, 0), 0)
	assert.Equal(t, OffsetForPosition(src, 0, 1), 1) // after 'a'
	assert.Equal(t, OffsetForPosition(src, 0, 3), 5) // after surrogate pair
}

func TestOffsetForPositionBytesUTF8(t *testing.T) {
	src := "a𐐷b"
	assert.Equal(t, OffsetForPositionBytes(src, 0, 0), 0)
	assert.Equal(t, OffsetForPositionBytes(src, 0, 5), 5)
}

func TestUTF16Len(t *testing.T) {
	assert.Equal(t, UTF16Len("hello"), 5)
	assert.Equal(t, UTF16Len("é"), 1)
	assert.Equal(t, UTF16Len("𐐷"), 2)
}

func TestWordAtOffset(t *testing.T) {
	src := "echo apt-get install"
	w, start, end := WordAtOffset(src, 7) // inside "apt-get"
	assert.Equal(t, w, "apt-get")
	assert.Equal(t, start, 5)
	assert.Equal(t, end, 12)

	w, _, _ = WordAtOffset(src, 4) // on space
	assert.Equal(t, w, "")

	w, _, _ = WordAtOffset(src, 100)
	assert.Equal(t, w, "")
}
