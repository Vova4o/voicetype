package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRecognizer struct{ samples []float32 }

func (f *fakeRecognizer) Accept(samples []float32) (string, bool, error) {
	f.samples = append(f.samples, samples...)
	return "привет", true, nil
}

func (f *fakeRecognizer) Finish() (string, error) { return "", nil }
func (f *fakeRecognizer) Close()                  {}

func TestPCM16ToFloat32(t *testing.T) {
	pcm := make([]byte, 6)
	binary.LittleEndian.PutUint16(pcm[0:], uint16(0))
	binary.LittleEndian.PutUint16(pcm[2:], uint16(16384))
	binary.LittleEndian.PutUint16(pcm[4:], uint16(32768))
	got := pcm16ToFloat32(pcm)
	want := []float32{0, 0.5, -1}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestModelPathsPrefersInt8(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"encoder.onnx", "encoder.int8.onnx", "decoder.onnx", "joiner.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := modelPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(files.encoder) != "encoder.int8.onnx" {
		t.Fatalf("encoder = %s", files.encoder)
	}
}

func TestModelPathsMissingFile(t *testing.T) {
	_, err := modelPaths(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "encoder") {
		t.Fatalf("error = %v", err)
	}
}

func TestListenPrintsRecognizedPhrase(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := filepath.Join(dir, "ffmpeg")
	if err := os.WriteFile(ffmpeg, []byte("#!/bin/sh\nprintf '\\000\\000\\000\\100'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	engine := &fakeRecognizer{}
	var output bytes.Buffer
	deliver := func(_ context.Context, text string) error {
		_, err := output.WriteString(text + "\n")
		return err
	}
	if err := listen(context.Background(), ":0", engine, deliver); err != nil {
		t.Fatal(err)
	}
	if output.String() != "привет\n" {
		t.Fatalf("output = %q", output.String())
	}
	if len(engine.samples) != 2 || engine.samples[0] != 0 || engine.samples[1] != 0.5 {
		t.Fatalf("samples = %v", engine.samples)
	}
}

func TestReadMicrophoneBuffersWhileDecoderIsBusy(t *testing.T) {
	input := bytes.NewReader(make([]byte, samplesPerChunk*2*3))
	chunks := make(chan []float32, 3)
	if err := readMicrophone(context.Background(), input, chunks); err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("buffered chunks = %d", len(chunks))
	}
}

func TestReadMicrophoneRejectsBacklogOverflow(t *testing.T) {
	chunks := make(chan []float32, 1)
	input := bytes.NewReader(make([]byte, samplesPerChunk*2*2))
	if err := readMicrophone(context.Background(), input, chunks); err == nil {
		t.Fatal("expected backlog error")
	}
}

func TestReadMicrophoneHandlesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := readMicrophone(ctx, bytes.NewReader([]byte{0, 0}), make(chan []float32, 1)); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadMicrophonePreservesReadError(t *testing.T) {
	if err := readMicrophone(context.Background(), failedAudioReader{}, make(chan []float32, 1)); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v", err)
	}
}

type failedAudioReader struct{}

func (failedAudioReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestListenCancelsBlockedCapture(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte("#!/bin/sh\n/bin/dd if=/dev/zero bs=3200 count=1 2>/dev/null\nexec /bin/sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- listen(ctx, ":default", &cancelRecognizer{cancel: cancel}, func(context.Context, string) error { return nil })
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("capture did not stop after cancellation")
	}
}

type cancelRecognizer struct{ cancel context.CancelFunc }

func (r *cancelRecognizer) Accept([]float32) (string, bool, error) { r.cancel(); return "", false, nil }
func (*cancelRecognizer) Finish() (string, error)                  { return "", nil }
func (*cancelRecognizer) Close()                                   {}
