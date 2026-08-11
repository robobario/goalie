package display

import "strings"

// NoteWord is a single word from a note with two representations. Original is
// the raw text used for rendering; Display is used for width calculation and
// equals CompressURL(Original) for URLs when hyperLinks is true, or Original
// in all other cases.
type NoteWord struct {
	Original string
	Display  string
	// Token is true when this word matched a URL or @mention pattern.
	Token bool
}

// TokenizeNoteWords parses note into a slice of NoteWord. Each space-separated
// word whose entire text matches a URL or @mention pattern gets Token=true; URL
// tokens additionally get Display set to CompressURL(url) when hyperLinks is
// true. Words that only partially match (e.g. "@alice's") are left as plain
// words so their adjacent punctuation is preserved during rendering.
func TokenizeNoteWords(note string, hyperLinks bool) []NoteWord {
	words := strings.Fields(note)
	result := make([]NoteWord, len(words))
	for i, w := range words {
		loc := statusNoteTokenRe.FindStringIndex(w)
		if loc != nil && loc[0] == 0 && loc[1] == len(w) {
			display := w
			if hyperLinks && strings.HasPrefix(w, "http") {
				display = CompressURL(w)
			}
			result[i] = NoteWord{Original: w, Display: display, Token: true}
		} else {
			result[i] = NoteWord{Original: w, Display: w}
		}
	}
	return result
}

// TakeFirstLineWords splits words into a first slice that fits within maxWidth
// display characters and a rest slice. When the first word alone exceeds
// maxWidth, first is nil and rest is all of words (matching the behaviour of
// the plain-string takeFirstLine).
func TakeFirstLineWords(words []NoteWord, maxWidth int) (first, rest []NoteWord) {
	if maxWidth <= 0 || len(words) == 0 {
		return nil, words
	}
	currentWidth := 0
	taken := 0
	for i, w := range words {
		dw := len(w.Display)
		if i == 0 {
			if dw > maxWidth {
				return nil, words
			}
			currentWidth = dw
			taken = i + 1
		} else if currentWidth+1+dw <= maxWidth {
			currentWidth += 1 + dw
			taken = i + 1
		} else {
			break
		}
	}
	return words[:taken], words[taken:]
}

// WrapNoteWords splits words into lines where each line's display width does
// not exceed maxWidth. A single word wider than maxWidth appears on its own
// line unsplit.
func WrapNoteWords(words []NoteWord, maxWidth int) [][]NoteWord {
	if maxWidth <= 0 || len(words) == 0 {
		return [][]NoteWord{words}
	}
	var lines [][]NoteWord
	remaining := words
	for len(remaining) > 0 {
		first, rest := TakeFirstLineWords(remaining, maxWidth)
		if len(first) == 0 {
			first = remaining[:1]
			rest = remaining[1:]
		}
		lines = append(lines, first)
		remaining = rest
	}
	return lines
}

// noteWordsWidth returns the total display width of words joined by single spaces.
func noteWordsWidth(words []NoteWord) int {
	total := 0
	for i, w := range words {
		if i > 0 {
			total++
		}
		total += len(w.Display)
	}
	return total
}
