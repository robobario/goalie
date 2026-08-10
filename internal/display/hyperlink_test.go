package display

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestCompressURL_GitHubPR(t *testing.T) {
	got := CompressURL("https://github.com/kroxylicious/kroxylicious/pull/1331")
	if got != "kroxylicious/kroxylicious#1331" {
		t.Errorf("got %q, want %q", got, "kroxylicious/kroxylicious#1331")
	}
}

func TestCompressURL_GitHubIssue(t *testing.T) {
	got := CompressURL("https://github.com/owner/repo/issues/42")
	if got != "owner/repo#42" {
		t.Errorf("got %q, want %q", got, "owner/repo#42")
	}
}

func TestCompressURL_NonGitHubShort(t *testing.T) {
	got := CompressURL("https://example.com")
	if got != "example.com" {
		t.Errorf("got %q, want %q", got, "example.com")
	}
}

func TestCompressURL_NonGitHubLong(t *testing.T) {
	got := CompressURL("https://www.example.com/thing/x/y/lonoooooooog")
	want := "www.example.com/thin..."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompressURL_NonGitHubExactly20(t *testing.T) {
	// 20-char path after stripping scheme — no ellipsis
	url := "https://" + strings.Repeat("a", 20)
	got := CompressURL(url)
	want := strings.Repeat("a", 20)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCompressURL_HTTP(t *testing.T) {
	got := CompressURL("http://example.com/path")
	if got != "example.com/path" {
		t.Errorf("got %q, want %q", got, "example.com/path")
	}
}

var noStyle = lipgloss.NewStyle()

func TestRenderURL_NoHyperLinks(t *testing.T) {
	got := RenderURL("https://github.com/a/b/pull/1", noStyle, false)
	if got != "https://github.com/a/b/pull/1" {
		t.Errorf("got %q", got)
	}
}

func TestRenderURL_HyperLinksGitHub(t *testing.T) {
	url := "https://github.com/a/b/pull/1"
	got := RenderURL(url, noStyle, true)
	if !strings.Contains(got, "a/b#1") {
		t.Errorf("expected compressed label in %q", got)
	}
	if !strings.Contains(got, url) {
		t.Errorf("expected full URL in OSC 8 target in %q", got)
	}
	if !strings.HasPrefix(got, "\033]8;;") {
		t.Errorf("expected OSC 8 prefix in %q", got)
	}
}

func TestRenderURL_HyperLinksNonGitHub(t *testing.T) {
	url := "https://www.example.com/thing/x/y/lonoooooooog"
	got := RenderURL(url, noStyle, true)
	if !strings.Contains(got, "www.example.com/thin...") {
		t.Errorf("expected truncated label in %q", got)
	}
	if !strings.Contains(got, url) {
		t.Errorf("expected full URL as OSC 8 target in %q", got)
	}
}
