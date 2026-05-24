// SPDX-License-Identifier: GPL-3.0-only

// Package lsp bridges LSP position/range coordinates to the byte offsets
// the rest of the codebase prefers. Anything touching protocol coordinates
// lives here.
package lsp

// isWordChar reports whether b is part of a "word" for hover/completion/
// definition lookup. Alphanumeric plus underscore, hyphen, period — so
// `apt-get`, `foo.bar`, and `_var` all resolve as a single identifier.
func isWordChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '-' || b == '.':
		return true
	}
	return false
}

// WordAtOffset returns the word surrounding the given byte offset in src,
// or "" if the offset is on a non-word byte or out of range. The end
// offset (one past the last byte of the word) is also returned to let
// callers build a precise Range without re-scanning.
func WordAtOffset(src string, off int) (word string, start, end int) {
	if off < 0 || off >= len(src) || !isWordChar(src[off]) {
		return "", 0, 0
	}
	start = off
	for start > 0 && isWordChar(src[start-1]) {
		start--
	}
	end = off
	for end < len(src) && isWordChar(src[end]) {
		end++
	}
	return src[start:end], start, end
}

// OffsetForPositionBytes is the UTF-8 / byte-offset counterpart to
// OffsetForPosition. When the client negotiated utf-8 PositionEncoding,
// "character" is already a byte count inside the line.
func OffsetForPositionBytes(src string, line, character int) int {
	if line < 0 || character < 0 {
		return -1
	}
	cur, curLine := lineStart(src, line)
	if cur < 0 {
		return -1
	}
	off := cur + character
	if off > len(src) {
		return -1
	}
	_ = curLine
	return off
}

// OffsetForPosition translates an LSP (line, character) under UTF-16
// encoding into a byte offset in src. Returns -1 if out of range.
func OffsetForPosition(src string, line, character int) int {
	if line < 0 || character < 0 {
		return -1
	}
	cur, curLine := lineStart(src, line)
	if cur < 0 {
		return -1
	}
	_ = curLine
	u16 := 0
	for cur < len(src) && u16 < character {
		b := src[cur]
		if b == '\n' {
			break
		}
		if b < 0x80 {
			cur++
			u16++
			continue
		}
		size, u16len := utf8RuneInfo(src[cur:])
		cur += size
		u16 += u16len
	}
	if u16 < character {
		// character past end-of-line is clamped to line end (common in
		// editor-reported positions, e.g. when the cursor is at column
		// 0 of a logically empty line).
		return cur
	}
	return cur
}

// UTF16Len returns the length of s measured in UTF-16 code units.
func UTF16Len(s string) int {
	total := 0
	for i := 0; i < len(s); {
		size, u16 := utf8RuneInfo(s[i:])
		if size == 0 {
			break
		}
		i += size
		total += u16
	}
	return total
}

// lineStart walks src and returns the byte offset of the start of `line`
// (0-based). If src has fewer lines, returns (-1, _).
func lineStart(src string, line int) (int, int) {
	cur, curLine := 0, 0
	for cur < len(src) && curLine < line {
		if src[cur] == '\n' {
			curLine++
		}
		cur++
	}
	if curLine != line {
		return -1, curLine
	}
	return cur, curLine
}

// utf8RuneInfo returns (byte length, UTF-16 code unit length) for the rune
// at the start of s. Invalid UTF-8 returns (1, 1) so callers can make
// progress.
func utf8RuneInfo(s string) (int, int) {
	if len(s) == 0 {
		return 0, 0
	}
	b0 := s[0]
	switch {
	case b0 < 0x80:
		return 1, 1
	case b0 < 0xC0:
		return 1, 1 // stray continuation byte
	case b0 < 0xE0:
		return 2, 1
	case b0 < 0xF0:
		return 3, 1
	default:
		return 4, 2 // codepoint >= U+10000 needs a UTF-16 surrogate pair
	}
}
