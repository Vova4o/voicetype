package main

import (
	"errors"
	"sync"
	"testing"
)

func TestDictationMenuToggleStartsInput(t *testing.T) {
	state := newDictationState(nil)
	state.setReady()
	state.toggle()
	if text, _ := state.accept("Проверяю ввод."); text != "Проверяю ввод." {
		t.Fatalf("text = %q", text)
	}
}
func TestDictationMenuToggleStopsInput(t *testing.T) {
	state := newDictationState(nil)
	state.setReady()
	state.accept("Енот текст")
	state.toggle()
	if text, _ := state.accept("Лишние слова"); text != "" {
		t.Fatalf("text = %q", text)
	}
}
func TestDictationMenuDisabledUntilReady(t *testing.T) {
	state := newDictationState(nil)
	state.toggle()
	if text, _ := state.accept("Лишние слова"); text != "" {
		t.Fatalf("text = %q", text)
	}
}
func TestDictationStateConcurrentMenuAndSpeech(t *testing.T) {
	state := newDictationState(nil)
	state.setReady()
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for i := 0; i < 100; i++ {
			state.toggle()
		}
	}()
	go func() {
		defer group.Done()
		for i := 0; i < 100; i++ {
			state.accept("Енот текст")
		}
	}()
	group.Wait()
}

func TestDictationClearsRecoveredPasteError(t *testing.T) {
	var problem string
	state := newDictationState(func(_ bool, _ bool, p string, _ string) { problem = p })
	state.setReady()
	state.fail(errors.New("paste denied"))
	state.pasted()
	if problem != "" {
		t.Fatalf("stale error = %q", problem)
	}
}
