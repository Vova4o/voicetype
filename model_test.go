package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureModelDownloadsOnce(t *testing.T) {
	data := []byte("model data")
	digest := sha256.Sum256(data)
	file := modelFile{name: "encoder.onnx", size: int64(len(data)), sha256: hex.EncodeToString(digest[:])}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.URL.Path != "/encoder.onnx" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if _, err := writer.Write(data); err != nil {
			t.Errorf("write model: %v", err)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := ensureModel(context.Background(), dir, server.Client(), server.URL+"/", []modelFile{file}); err != nil {
			t.Fatal(err)
		}
	}
	if requests != 1 {
		t.Fatalf("requests = %d", requests)
	}
	got, err := os.ReadFile(filepath.Join(dir, file.name))
	if err != nil || string(got) != string(data) {
		t.Fatalf("model = %q, error = %v", got, err)
	}
}

func TestEnsureModelRejectsBadChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if _, err := writer.Write([]byte("bad")); err != nil {
			t.Errorf("write model: %v", err)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	file := modelFile{name: "encoder.onnx", size: 3, sha256: strings.Repeat("0", 64)}
	err := ensureModel(context.Background(), dir, server.Client(), server.URL+"/", []modelFile{file})
	if err == nil || !strings.Contains(err.Error(), "контрольная сумма") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, file.name)); !os.IsNotExist(err) {
		t.Fatalf("incomplete model installed: %v", err)
	}
}

func TestEnsureModelHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := ensureModel(ctx, t.TempDir(), http.DefaultClient, "http://127.0.0.1/", []modelFile{{name: "model.onnx"}})
	if err == nil || !strings.Contains(err.Error(), "отменена") {
		t.Fatalf("error = %v", err)
	}
}

func TestModelReadyRequiresEveryFile(t *testing.T) {
	dir := t.TempDir()
	files := []modelFile{{name: "encoder.onnx", size: 1}, {name: "tokens.txt", size: 1}}
	if err := os.WriteFile(filepath.Join(dir, files[0].name), []byte("a"), 0600); err != nil {
		t.Fatal(err)
	}
	ready, err := modelReady(dir, files)
	if err != nil || ready {
		t.Fatalf("incomplete model: ready=%v, error=%v", ready, err)
	}
	if err := os.WriteFile(filepath.Join(dir, files[1].name), []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	ready, err = modelReady(dir, files)
	if err != nil || !ready {
		t.Fatalf("complete model: ready=%v, error=%v", ready, err)
	}
}
