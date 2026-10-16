package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCaptureReconnectsAfterStalledFFmpeg(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
if [ -f "$CAPTURE_ATTEMPT_FILE" ]; then
 /bin/dd if=/dev/zero bs=3200 count=1 2>/dev/null
else
 : > "$CAPTURE_ATTEMPT_FILE"
 /bin/dd if=/dev/zero bs=3200 count=1 2>/dev/null
 exec /bin/sleep 60
fi
`
	if err := os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("CAPTURE_ATTEMPT_FILE", filepath.Join(dir, "attempt"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	attempts, recoveries, readyCount, delivered := 0, 0, 0, 0
	err := superviseCapture(ctx, func(ctx context.Context) error {
		attempts++
		return listenCapture(ctx, ":default", &fakeRecognizer{}, func(context.Context, string) error {
			delivered++
			if attempts == 2 {
				cancel()
			}
			return nil
		}, time.Second, func() { readyCount++ })
	}, func(err error) {
		recoveries++
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("recovery error = %v", err)
		}
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || recoveries != 1 || readyCount != 2 || delivered != 2 {
		t.Fatalf("attempts=%d recoveries=%d ready=%d delivered=%d", attempts, recoveries, readyCount, delivered)
	}
}

func TestCaptureDoesNotRetryDecoderFailure(t *testing.T) {
	failure := errors.New("decoder failed")
	err := superviseCapture(context.Background(), func(context.Context) error { return failure }, func(error) { t.Fatal("retried a decoder failure") }, 0)
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
}

func TestCaptureCancellationStopsRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts := 0
	err := superviseCapture(ctx, func(context.Context) error {
		attempts++
		return &captureFailure{errors.New("device unavailable")}
	}, func(error) { cancel() }, time.Hour)
	if err != nil || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
}

func TestCaptureRestartsUnexpectedEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	retried := false
	err := superviseCapture(ctx, func(context.Context) error { return nil }, func(error) { retried = true; cancel() }, 0)
	if err != nil || !retried {
		t.Fatalf("retried=%t error=%v", retried, err)
	}
}
