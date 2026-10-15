package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVoskMissingLibraryReturnsError(t *testing.T) {
	engine, err := newVosk(filepath.Join(t.TempDir(), "missing.dylib"), modelFiles{}, 1)
	if engine != nil || err == nil || !strings.Contains(err.Error(), "загрузить vosk") {
		t.Fatalf("engine=%v error=%v", engine, err)
	}
}

func TestVoskClosedStreamRejectsAudio(t *testing.T) {
	engine := &vosk{}
	engine.Close()
	if _, _, err := engine.Accept([]float32{0}); err == nil {
		t.Fatal("closed stream accepted audio")
	}
}
