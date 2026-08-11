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

// TokenizeNoteWords parses note into a slice of NoteWord. URL tokens get
// Display set to CompressURL(url) when hyperLinks is true; all other words
// carry Display equal to Original.
func TokenizeNoteWords(note string, hyperLinks bool) []NoteWord {
	var words []NoteWord
	last := 0
	for _, loc := range statusNoteTokenRe.FindAllStringIndex(note, -1) {
		if loc[0] > last {
			for _, w := range strings.Fields(note[last:loc[0]]) {
				words = append(words, NoteWord{Original: w, Display: w})
			}
		}
		m := note[loc[0]:loc[1]]
		display := m
		if hyperLinks && strings.HasPrefix(m, "http") {
			display = CompressURL(m)
		}
		words = append(words, NoteWord{Original: m, Display: display, Token: true})
		last = loc[1]
	}
	for _, w := range strings.Fields(note[last:]) {
		words = append(words, NoteWord{Original: w, Display: w})
	}
	return words
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
