package display

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const urlCompressMaxLen = 20

var githubURLRe = regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)/(pull|issues)/(\d+)`)

// CompressURL returns a shortened representation of url suitable for display
// in a terminal. GitHub PR and issue URLs are compressed to "owner/repo#num".
// All other URLs have their scheme stripped; if the result exceeds
// urlCompressMaxLen characters it is truncated with a trailing ellipsis.
func CompressURL(url string) string {
	if m := githubURLRe.FindStringSubmatch(url); m != nil {
		return m[1] + "/" + m[2] + "#" + m[4]
	}
	stripped := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	if len(stripped) > urlCompressMaxLen {
		return stripped[:urlCompressMaxLen] + "..."
	}
	return stripped
}

// RenderURL renders url as a coloured, optionally hyperlinked token.
// When hyperLinks is true the visible label is compressed via CompressURL and
// wrapped in an OSC 8 terminal hyperlink pointing at the original url.
// style is applied to the visible label in both modes.
func RenderURL(url string, style lipgloss.Style, hyperLinks bool) string {
	if !hyperLinks {
		return style.Render(url)
	}
	label := CompressURL(url)
	colored := style.Render(label)
	return "\033]8;;" + url + "\033\\" + colored + "\033]8;;\033\\"
}
