package main

import "testing"

func TestStreamingTranscriptDoesNotRepeatJoinedCommandAfterSplit(t *testing.T) {
	var transcript streamingTranscript
	for range 3 {
		transcript.Update("енотекст проверяю", false)
	}
	for range 3 {
		if delta := transcript.Update("енот текст проверяю", false); delta != "" {
			t.Fatalf("command repeated after boundary revision: %q", delta)
		}
	}
	if delta := transcript.Update("енот текст проверяю ввод", true); delta != "проверяю ввод" {
		t.Fatalf("final delta=%q", delta)
	}
}

func TestStreamingTranscriptPreservesWakeAfterWordResegmentation(t *testing.T) {
	var transcript streamingTranscript
	gate := newVoiceGate("Енот текст", "Енот стоп")
	started := false
	for _, hypothesis := range []string{
		"и нот проверяю", "и нот проверяю", "и нот проверяю",
		"енот текст проверяю", "енот текст проверяю", "енот текст проверяю",
	} {
		if delta := transcript.Update(hypothesis, false); delta != "" {
			_, event := gate.Accept(delta)
			started = started || event == gateStarted
		}
	}
	if !started {
		t.Fatal("wake command was lost when two words became one")
	}
}

func TestStreamingTranscriptEmitsStableWordsBeforeEndpoint(t *testing.T) {
	var transcript streamingTranscript
	for _, text := range []string{"енот текст проверяю", "енот текст проверяю ввод"} {
		if got := transcript.Update(text, false); got != "" {
			t.Fatalf("unstable words emitted: %q", got)
		}
	}
	if got := transcript.Update("енот текст проверяю ввод", false); got != "енот текст проверяю" {
		t.Fatalf("stable delta=%q", got)
	}
}

func TestStreamingTranscriptDoesNotDuplicateWordsAtEndpoint(t *testing.T) {
	var transcript streamingTranscript
	for range 3 {
		transcript.Update("проверяю ввод", false)
	}
	if got := transcript.Update("проверяю ввод", true); got != "ввод" {
		t.Fatalf("final delta=%q", got)
	}
	if got := transcript.Update("следующая фраза", true); got != "следующая фраза" {
		t.Fatalf("next utterance=%q", got)
	}
}

func TestStreamingTranscriptWithholdsRevisedPartialWords(t *testing.T) {
	var transcript streamingTranscript
	transcript.Update("енотек проверяю", false)
	transcript.Update("енотекст проверяю", false)
	if got := transcript.Update("енот текст проверяю", false); got != "" {
		t.Fatalf("revised partial emitted: %q", got)
	}
}
