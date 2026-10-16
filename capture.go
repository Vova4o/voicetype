package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

const microphoneTimeout = 10 * time.Second

type captureFailure struct{ cause error }

func (e *captureFailure) Error() string { return e.cause.Error() }
func (e *captureFailure) Unwrap() error { return e.cause }

// microphoneReader bounds each pipe read, including reads after device loss.
type microphoneReader struct {
	pipe    *os.File
	timeout time.Duration
}

func (r microphoneReader) Read(buffer []byte) (int, error) {
	if err := r.pipe.SetReadDeadline(time.Now().Add(r.timeout)); err != nil {
		return 0, fmt.Errorf("задать таймаут микрофона: %w", err)
	}
	return r.pipe.Read(buffer)
}

func superviseCapture(ctx context.Context, attempt func(context.Context) error, recovering func(error), retryDelay time.Duration) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		err := attempt(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			err = &captureFailure{errors.New("поток микрофона неожиданно завершился")}
		}
		var failure *captureFailure
		if !errors.As(err, &failure) {
			return err
		}
		recovering(err)
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
