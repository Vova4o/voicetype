package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		}, time.Second, func() { readyCount++ }, nil)
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

func TestInputDeviceCheckDetectsChangeWithLiveAudio(t *testing.T) {
	current := uint32(1)
	check, err := inputDeviceCheck(func() (uint32, error) { return current, nil })
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if _, err := writer.Write([]byte{1, 0, 2, 0}); err != nil {
		t.Fatal(err)
	}
	input := microphoneReader{pipe: reader, timeout: time.Second, checkDevice: check}
	buffer := make([]byte, 2)
	if _, err := input.Read(buffer); err != nil {
		t.Fatal(err)
	}
	current = 2
	if _, err := input.Read(buffer); err == nil || !strings.Contains(err.Error(), "системный микрофон изменился") {
		t.Fatalf("device change was ignored: %v", err)
	}
}

func TestInputDeviceCheckKeepsExistingDevice(t *testing.T) {
	check, err := inputDeviceCheck(func() (uint32, error) { return 42, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := check(); err != nil {
		t.Fatal(err)
	}
}

func TestInputDeviceCheckReportsUnavailableDevice(t *testing.T) {
	failure := errors.New("no input device")
	if _, err := inputDeviceCheck(func() (uint32, error) { return 0, failure }); !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
}
