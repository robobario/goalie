package display

import (
	"testing"
)

func TestTokenizeNoteWordsPlainText(t *testing.T) {
	words := TokenizeNoteWords("hello world", false)
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d: %v", len(words), words)
	}
	if words[0].Original != "hello" || words[0].Display != "hello" || words[0].Token {
		t.Errorf("unexpected first word: %+v", words[0])
	}
	if words[1].Original != "world" || words[1].Display != "world" || words[1].Token {
		t.Errorf("unexpected second word: %+v", words[1])
	}
}

func TestTokenizeNoteWordsURLHyperLinksOff(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42"
	words := TokenizeNoteWords("see "+url, false)
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
	urlWord := words[1]
	if urlWord.Original != url {
		t.Errorf("original = %q, want %q", urlWord.Original, url)
	}
	if urlWord.Display != url {
		t.Errorf("display should equal original when hyperLinks=false, got %q", urlWord.Display)
	}
	if !urlWord.Token {
		t.Errorf("URL word should have Token=true")
	}
}

func TestTokenizeNoteWordsURLHyperLinksOn(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42"
	words := TokenizeNoteWords("see "+url, true)
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
	urlWord := words[1]
	if urlWord.Original != url {
		t.Errorf("original = %q, want %q", urlWord.Original, url)
	}
	want := "owner/repo#42"
	if urlWord.Display != want {
		t.Errorf("display = %q, want compressed %q", urlWord.Display, want)
	}
	if !urlWord.Token {
		t.Errorf("URL word should have Token=true")
	}
}

func TestTokenizeNoteWordsMention(t *testing.T) {
	words := TokenizeNoteWords("ping @alice", false)
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
	if words[1].Original != "@alice" || !words[1].Token {
		t.Errorf("unexpected mention word: %+v", words[1])
	}
}

func TestTokenizeNoteWordsPartialMention(t *testing.T) {
	// "@alice's" partially matches @mention; the apostrophe is not consumed.
	// The whole word must be Token=false so adjacent punctuation is not split.
	words := TokenizeNoteWords("@alice's work", false)
	if len(words) != 2 {
		t.Fatalf("expected 2 words, got %d: %v", len(words), words)
	}
	if words[0].Token {
		t.Errorf("partial-match word should have Token=false: %+v", words[0])
	}
	if words[0].Original != "@alice's" {
		t.Errorf("original should be '@alice's', got %q", words[0].Original)
	}
}

func TestTakeFirstLineWordsAllFit(t *testing.T) {
	words := TokenizeNoteWords("one two three", false)
	first, rest := TakeFirstLineWords(words, 80)
	if len(first) != 3 {
		t.Errorf("expected all 3 words in first, got %d", len(first))
	}
	if len(rest) != 0 {
		t.Errorf("expected empty rest, got %d words", len(rest))
	}
}

func TestTakeFirstLineWordsSplits(t *testing.T) {
	words := TokenizeNoteWords("one two three four", false)
	first, rest := TakeFirstLineWords(words, 7) // "one two" = 7
	if len(first) != 2 {
		t.Errorf("expected 2 words in first, got %d", len(first))
	}
	if len(rest) != 2 {
		t.Errorf("expected 2 words in rest, got %d", len(rest))
	}
}

func TestTakeFirstLineWordsOversizedFirstWord(t *testing.T) {
	words := TokenizeNoteWords("toolongword next", false)
	first, rest := TakeFirstLineWords(words, 5)
	if len(first) != 0 {
		t.Errorf("expected empty first for oversized word, got %d words", len(first))
	}
	if len(rest) != 2 {
		t.Errorf("expected 2 words in rest, got %d", len(rest))
	}
}

func TestTakeFirstLineWordsURLDisplayWidth(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42" // raw: 37 chars, compressed: "owner/repo#42" = 13
	words := TokenizeNoteWords("see "+url+" done", true)
	// "see" (3) + " " + "owner/repo#42" (13) + " " + "done" (4) = 21
	// maxWidth 18: fits "see owner/repo#42" (3+1+13=17)
	first, rest := TakeFirstLineWords(words, 18)
	if len(first) != 2 {
		t.Errorf("expected 2 words (see + compressed URL) in first, got %d: %v", len(first), first)
	}
	if len(rest) != 1 {
		t.Errorf("expected 1 word in rest, got %d", len(rest))
	}
}

func TestWrapNoteWordsNoWrap(t *testing.T) {
	words := TokenizeNoteWords("short text", false)
	lines := WrapNoteWords(words, 80)
	if len(lines) != 1 || len(lines[0]) != 2 {
		t.Errorf("expected single line with 2 words, got %v", lines)
	}
}

func TestWrapNoteWordsWraps(t *testing.T) {
	words := TokenizeNoteWords("one two three four", false)
	lines := WrapNoteWords(words, 7) // "one two"=7, "three"=5, "four"=4
	if len(lines) < 2 {
		t.Errorf("expected multiple lines, got %d", len(lines))
	}
	for _, line := range lines {
		w := noteWordsWidth(line)
		if w > 7 {
			t.Errorf("line display width %d exceeds maxWidth 7: %v", w, line)
		}
	}
}

func TestWrapNoteWordsOversizedWordOnOwnLine(t *testing.T) {
	words := TokenizeNoteWords("toolongword next", false)
	lines := WrapNoteWords(words, 5)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0][0].Original != "toolongword" {
		t.Errorf("expected oversized word on first line, got %q", lines[0][0].Original)
	}
}

func TestWrapNoteWordsURLCompressedWidth(t *testing.T) {
	url := "https://github.com/owner/repo/pull/42" // compressed: "owner/repo#42" = 13
	words := TokenizeNoteWords("word "+url+" end", true)
	// display widths: "word"=4, "owner/repo#42"=13, "end"=3
	// maxWidth 20: "word owner/repo#42" = 4+1+13 = 18 fits; adding "end" = 22 doesn't
	lines := WrapNoteWords(words, 20)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}
