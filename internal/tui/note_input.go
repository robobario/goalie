package tui

import (
	"strings"
	"unicode/utf8"
)

// noteInput is a UTF-8 aware text field with a byte-offset cursor.
type noteInput struct {
	value  string
	cursor int // byte offset; always on a rune boundary
}

func newNoteInput(value string) noteInput {
	return noteInput{value: value, cursor: len(value)}
}

func (n noteInput) atEnd() bool {
	return n.cursor == len(n.value)
}

func (n noteInput) insert(r rune) noteInput {
	encoded := string(r)
	n.value = n.value[:n.cursor] + encoded + n.value[n.cursor:]
	n.cursor += len(encoded)
	return n
}

func (n noteInput) backspace() noteInput {
	if n.cursor == 0 {
		return n
	}
	_, size := utf8.DecodeLastRuneInString(n.value[:n.cursor])
	n.value = n.value[:n.cursor-size] + n.value[n.cursor:]
	n.cursor -= size
	return n
}

func (n noteInput) left() noteInput {
	if n.cursor == 0 {
		return n
	}
	_, size := utf8.DecodeLastRuneInString(n.value[:n.cursor])
	n.cursor -= size
	return n
}

func (n noteInput) right() noteInput {
	if n.cursor == len(n.value) {
		return n
	}
	_, size := utf8.DecodeRuneInString(n.value[n.cursor:])
	n.cursor += size
	return n
}

func (n noteInput) appendStr(s string) noteInput {
	n.value = n.value[:n.cursor] + s + n.value[n.cursor:]
	n.cursor += len(s)
	return n
}

// view renders the field content with a _ cursor inserted at the current
// position. Mention highlighting is applied to each half independently.
func (n noteInput) view(username string) string {
	before := renderNoteWithMentions(n.value[:n.cursor], username, false)
	after := renderNoteWithMentions(n.value[n.cursor:], username, false)
	return before + "_" + after
}

// viewWrapped is like view but word-wraps the content at maxWidth columns.
// The cursor _ is embedded in the value before wrapping so it stays on the
// correct visual line. When maxWidth <= 0 it falls back to view.
func (n noteInput) viewWrapped(username string, maxWidth int) string {
	if maxWidth <= 0 {
		return n.view(username)
	}
	display := n.value[:n.cursor] + "_" + n.value[n.cursor:]
	lines := wrapWords(display, maxWidth)
	parts := make([]string, len(lines))
	for i, line := range lines {
		parts[i] = renderNoteWithMentions(line, username, false)
	}
	return strings.Join(parts, "\n")
}
