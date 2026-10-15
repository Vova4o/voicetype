package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func pasteText(ctx context.Context, text string) error {
	return pasteTextWithKeypress(ctx, text, pressCommandV)
}

func pasteTextWithKeypress(ctx context.Context, text string, keypress func() error) error {
	copyCtx, cancelCopy := context.WithTimeout(ctx, 5*time.Second)
	defer cancelCopy()
	copyCommand := exec.CommandContext(copyCtx, "pbcopy")
	copyCommand.Env = append(os.Environ(), "LC_ALL=en_US.UTF-8")
	copyCommand.Stdin = strings.NewReader(text + " ")
	if output, err := copyCommand.CombinedOutput(); err != nil {
		return fmt.Errorf("скопировать текст в буфер обмена: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("вставить текст: %w; текст остался в буфере обмена", err)
	}
	if err := keypress(); err != nil {
		return fmt.Errorf("нажать ⌘V: %w; текст остался в буфере обмена", err)
	}
	return nil
}
