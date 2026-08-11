package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func intPtr(i int) *int { return &i }

func boolPtr(b bool) *bool { return &b }

func TestEffectiveCompressHyperLinksDefault(t *testing.T) {
	cfg := &Config{Name: "test"}
	if cfg.EffectiveCompressHyperLinks() {
		t.Error("expected false when CompressHyperLinks is nil")
	}
}

func TestEffectiveCompressHyperLinksTrue(t *testing.T) {
	cfg := &Config{Name: "test", CompressHyperLinks: boolPtr(true)}
	if !cfg.EffectiveCompressHyperLinks() {
		t.Error("expected true when CompressHyperLinks is true")
	}
}

func TestEffectiveCompressHyperLinksFalse(t *testing.T) {
	cfg := &Config{Name: "test", CompressHyperLinks: boolPtr(false)}
	if cfg.EffectiveCompressHyperLinks() {
		t.Error("expected false when CompressHyperLinks is false")
	}
}

func TestCompressHyperLinksOmittedWhenNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "test"}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "compress_hyperlinks") {
		t.Errorf("expected compress_hyperlinks omitted when nil, got: %s", data)
	}
}

func TestCompressHyperLinksRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "test", CompressHyperLinks: boolPtr(true)}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.CompressHyperLinks == nil || !*got.CompressHyperLinks {
		t.Errorf("CompressHyperLinks round-trip failed: got %v", got.CompressHyperLinks)
	}
}

func TestEffectiveNotificationsDefault(t *testing.T) {
	cfg := &Config{Name: "test"}
	if cfg.EffectiveNotifications() {
		t.Error("expected false when Notifications is nil")
	}
}

func TestEffectiveNotificationsTrue(t *testing.T) {
	cfg := &Config{Name: "test", Notifications: boolPtr(true)}
	if !cfg.EffectiveNotifications() {
		t.Error("expected true when Notifications is true")
	}
}

func TestEffectiveNotificationsFalse(t *testing.T) {
	cfg := &Config{Name: "test", Notifications: boolPtr(false)}
	if cfg.EffectiveNotifications() {
		t.Error("expected false when Notifications is false")
	}
}

func TestNotificationsOmittedWhenNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "test"}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "notifications") {
		t.Errorf("expected notifications omitted when nil, got: %s", data)
	}
}

func TestNotificationsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "test", Notifications: boolPtr(true)}
	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Notifications == nil || !*got.Notifications {
		t.Errorf("Notifications round-trip failed: got %v", got.Notifications)
	}
}

func TestEffectiveStatusDaysDefault(t *testing.T) {
	cfg := &Config{Name: "test"}
	if got := cfg.EffectiveStatusDays(); got != DefaultStatusDays {
		t.Errorf("got %d, want %d", got, DefaultStatusDays)
	}
}

func TestEffectiveStatusDaysCustom(t *testing.T) {
	cfg := &Config{Name: "test", StatusDays: intPtr(14)}
	if got := cfg.EffectiveStatusDays(); got != 14 {
		t.Errorf("got %d, want 14", got)
	}
}

func TestEffectiveWrapWidthDefault(t *testing.T) {
	cfg := &Config{Name: "test"}
	if got := cfg.EffectiveWrapWidth(); got != 120 {
		t.Errorf("got %d, want 120", got)
	}
}

func TestEffectiveWrapWidthCustom(t *testing.T) {
	cfg := &Config{Name: "test", WrapWidth: intPtr(100)}
	if got := cfg.EffectiveWrapWidth(); got != 100 {
		t.Errorf("got %d, want 100", got)
	}
}

func TestWrapWidthRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "test", WrapWidth: intPtr(80)}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.WrapWidth == nil || *got.WrapWidth != 80 {
		t.Errorf("WrapWidth round-trip failed: got %v", got.WrapWidth)
	}
}

func TestWrapWidthOmittedWhenNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "test"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "wrap_width") {
		t.Errorf("expected wrap_width omitted when nil, got: %s", data)
	}
}

func TestValidUsername(t *testing.T) {
	valid := []string{"@alice", "@Alice", "@alice-jones", "@a", "@a1b2c3"}
	for _, v := range valid {
		if !ValidUsername(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"alice", "@", "@-alice", "@alice!", "@ alice", ""}
	for _, v := range invalid {
		if ValidUsername(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "my-repo"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Name != cfg.Name {
		t.Errorf("got Name %q, want %q", got.Name, cfg.Name)
	}
}

func TestLoadFromMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := LoadFrom(path)
	if !errors.Is(err, ErrNotInitialised) {
		t.Errorf("got %v, want ErrNotInitialised", err)
	}
}

func TestSaveToCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subdir", "config.json")
	cfg := &Config{Name: "test"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestWrittenJSONIsValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &Config{Name: "valid-json"}

	if err := SaveTo(path, cfg); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("invalid JSON: %v", err)
	}
}
