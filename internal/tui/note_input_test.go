package tui

import (
	"testing"
)

func TestNoteInputInsertAtEnd(t *testing.T) {
	n := noteInput{}
	n = n.insert('h').insert('i')
	if n.value != "hi" {
		t.Errorf("expected 'hi', got %q", n.value)
	}
	if n.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", n.cursor)
	}
}

func TestNoteInputInsertAtMiddle(t *testing.T) {
	n := newNoteInput("hllo")
	n.cursor = 1 // after 'h'
	n = n.insert('e')
	if n.value != "hello" {
		t.Errorf("expected 'hello', got %q", n.value)
	}
	if n.cursor != 2 {
		t.Errorf("expected cursor=2 after insert, got %d", n.cursor)
	}
}

func TestNoteInputBackspaceAtEnd(t *testing.T) {
	n := newNoteInput("hello")
	n = n.backspace()
	if n.value != "hell" {
		t.Errorf("expected 'hell', got %q", n.value)
	}
	if n.cursor != 4 {
		t.Errorf("expected cursor=4, got %d", n.cursor)
	}
}

func TestNoteInputBackspaceAtMiddle(t *testing.T) {
	n := newNoteInput("hello")
	n.cursor = 3 // after 'l'
	n = n.backspace()
	if n.value != "helo" {
		t.Errorf("expected 'helo', got %q", n.value)
	}
	if n.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", n.cursor)
	}
}

func TestNoteInputBackspaceAtStart(t *testing.T) {
	n := newNoteInput("hi")
	n.cursor = 0
	n = n.backspace()
	if n.value != "hi" {
		t.Errorf("expected value unchanged, got %q", n.value)
	}
	if n.cursor != 0 {
		t.Errorf("expected cursor unchanged at 0, got %d", n.cursor)
	}
}

func TestNoteInputLeft(t *testing.T) {
	n := newNoteInput("hello")
	n = n.left().left()
	if n.cursor != 3 {
		t.Errorf("expected cursor=3 after two lefts, got %d", n.cursor)
	}
}

func TestNoteInputLeftAtStart(t *testing.T) {
	n := newNoteInput("hi")
	n.cursor = 0
	n = n.left()
	if n.cursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", n.cursor)
	}
}

func TestNoteInputRight(t *testing.T) {
	n := newNoteInput("hello")
	n.cursor = 0
	n = n.right().right()
	if n.cursor != 2 {
		t.Errorf("expected cursor=2 after two rights, got %d", n.cursor)
	}
}

func TestNoteInputRightAtEnd(t *testing.T) {
	n := newNoteInput("hi")
	n = n.right()
	if n.cursor != 2 {
		t.Errorf("expected cursor clamped at end, got %d", n.cursor)
	}
}

func TestNoteInputMultibyteLeft(t *testing.T) {
	// "é" is U+00E9, encoded as 2 bytes in UTF-8
	n := newNoteInput("café")
	// cursor starts at 6 (4 ASCII bytes + 2 for é... wait: c=1 a=1 f=1 é=2 = 5 bytes + 1 for the last 'é'? No: "café" = c(1)+a(1)+f(1)+é(2) = 5 bytes
	// Actually "café": c, a, f, é. é = U+00E9 = 0xC3 0xA9 = 2 bytes. So total = 1+1+1+2 = 5 bytes. cursor starts at 5.
	if n.cursor != 5 {
		t.Fatalf("expected cursor=5 for 'café', got %d (value len=%d)", n.cursor, len(n.value))
	}
	n = n.left()
	if n.cursor != 3 {
		t.Errorf("expected cursor=3 after left over 'é', got %d", n.cursor)
	}
}

func TestNoteInputMultibyteBackspace(t *testing.T) {
	// Backspace over a 2-byte rune should remove both bytes and move cursor back 2.
	n := newNoteInput("café")
	n = n.backspace()
	if n.value != "caf" {
		t.Errorf("expected 'caf' after backspace over 'é', got %q", n.value)
	}
	if n.cursor != 3 {
		t.Errorf("expected cursor=3, got %d", n.cursor)
	}
}

func TestNoteInputAtEnd(t *testing.T) {
	n := newNoteInput("hello")
	if !n.atEnd() {
		t.Error("expected atEnd() true for fresh noteInput")
	}
	n = n.left()
	if n.atEnd() {
		t.Error("expected atEnd() false after moving left")
	}
}

func TestNoteInputAppendStrAtEnd(t *testing.T) {
	n := newNoteInput("hello")
	n = n.appendStr(" world")
	if n.value != "hello world" {
		t.Errorf("expected 'hello world', got %q", n.value)
	}
	if n.cursor != len("hello world") {
		t.Errorf("expected cursor at end, got %d", n.cursor)
	}
}

func TestNoteInputAppendStrAtMiddle(t *testing.T) {
	n := newNoteInput("ac")
	n.cursor = 1 // after 'a'
	n = n.appendStr("b")
	if n.value != "abc" {
		t.Errorf("expected 'abc', got %q", n.value)
	}
	if n.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", n.cursor)
	}
}

func TestNoteInputViewCursorAtEnd(t *testing.T) {
	n := newNoteInput("hello")
	got := n.view("")
	if got != "hello_" {
		t.Errorf("expected 'hello_', got %q", got)
	}
}

func TestNoteInputViewCursorAtMiddle(t *testing.T) {
	n := newNoteInput("hello")
	n.cursor = 3
	got := n.view("")
	if got != "hel_lo" {
		t.Errorf("expected 'hel_lo', got %q", got)
	}
}

func TestNoteInputViewCursorAtStart(t *testing.T) {
	n := newNoteInput("hello")
	n.cursor = 0
	got := n.view("")
	if got != "_hello" {
		t.Errorf("expected '_hello', got %q", got)
	}
}
