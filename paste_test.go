package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPasteTextUsesCommandV(t *testing.T) {
	dir := t.TempDir()
	clipboard := filepath.Join(dir, "clipboard")
	locale := filepath.Join(dir, "locale")
	copyScript := "#!/bin/sh\nprintf '%s' \"$LC_ALL\" > '" + locale + "'\n/bin/cat > '" + clipboard + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "pbcopy"), []byte(copyScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("LC_ALL", "C")
	keyPressed := false
	if err := pasteTextWithKeypress(context.Background(), "привет", func() error {
		keyPressed = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	gotClipboard, err := os.ReadFile(clipboard)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotClipboard) != "привет " {
		t.Fatalf("clipboard = %q", gotClipboard)
	}
	gotLocale, err := os.ReadFile(locale)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotLocale) != "en_US.UTF-8" {
		t.Fatalf("locale = %q", gotLocale)
	}
	if !keyPressed {
		t.Fatal("⌘V was not pressed")
	}
}

func TestPasteTextPreservesClipboardOnPasteError(t *testing.T) {
	dir := t.TempDir()
	clipboard := filepath.Join(dir, "clipboard")
	copyScript := "#!/bin/sh\n/bin/cat > '" + clipboard + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "pbcopy"), []byte(copyScript), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	err := pasteTextWithKeypress(context.Background(), "текст", func() error {
		return os.ErrPermission
	})
	if err == nil || !strings.Contains(err.Error(), "буфере обмена") {
		t.Fatalf("error = %v", err)
	}
	got, err := os.ReadFile(clipboard)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "текст " {
		t.Fatalf("clipboard = %q", got)
	}
}
