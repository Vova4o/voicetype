package main

import "testing"

func TestVoiceGateAcceptsPhoneticWakeName(t *testing.T) {
	for _, command := range []string{"и нот текст", "и нотекст", "инот текст", "енод текст", "е нот текст", "и нот текс"} {
		t.Run(command, func(t *testing.T) {
			gate := newVoiceGate("Енот текст", "Енот стоп")
			text, event := gate.Accept(command + " проверяю ввод")
			if text != "проверяю ввод" || event != gateStarted {
				t.Fatalf("got %q event=%d", text, event)
			}
		})
	}
}

func TestVoiceGateAcceptsSplitPhoneticWakeName(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	for _, text := range []string{"и", "нот"} {
		if payload, event := gate.Accept(text); payload != "" || event != gateUnchanged {
			t.Fatalf("prefix %q: payload=%q event=%d", text, payload, event)
		}
	}
	if text, event := gate.Accept("текст проверяю ввод"); text != "проверяю ввод" || event != gateStarted {
		t.Fatalf("got %q event=%d", text, event)
	}
}

func TestVoiceGateAcceptsSplitPhoneticStopName(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	gate.Accept("енот текст")
	if text, _ := gate.Accept("это всё и"); text != "это всё" {
		t.Fatalf("dictation=%q", text)
	}
	if text, event := gate.Accept("нот стоп лишний текст"); text != "" || event != gateStopped || gate.active {
		t.Fatalf("stop: text=%q event=%d active=%t", text, event, gate.active)
	}
}

func TestVoiceGateReleasesPrefixWhenNextWordIsNotCommand(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	gate.Accept("енот текст")
	if text, _ := gate.Accept("проверяю и"); text != "проверяю" {
		t.Fatalf("dictation=%q", text)
	}
	if text, event := gate.Accept("продолжаю"); text != "и продолжаю" || event != gateUnchanged {
		t.Fatalf("text=%q event=%d", text, event)
	}
}

func TestVoiceGateRejectsCommonTextPhrases(t *testing.T) {
	for _, text := range []string{"этот текст", "и вот текст", "на этот текст", "я напишу текст", "нотный текст"} {
		gate := newVoiceGate("Енот текст", "Енот стоп")
		if payload, event := gate.Accept(text); payload != "" || event != gateUnchanged || gate.active {
			t.Fatalf("false activation for %q: payload=%q event=%d", text, payload, event)
		}
	}
}

func TestVoiceGateAcceptsWakeCommandSplitBetweenChunks(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	if text, event := gate.Accept("енот"); text != "" || event != gateUnchanged {
		t.Fatalf("first chunk: %q, %d", text, event)
	}
	if text, event := gate.Accept("текст проверяю"); text != "проверяю" || event != gateStarted {
		t.Fatalf("second chunk: %q, %d", text, event)
	}
}

func TestVoiceGateWithholdsStopPrefixUntilNextChunk(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	gate.Accept("енот текст")
	if text, event := gate.Accept("это всё енот"); text != "это всё" || event != gateUnchanged {
		t.Fatalf("first chunk: %q, %d", text, event)
	}
	if text, event := gate.Accept("стоп лишние слова"); text != "" || event != gateStopped || gate.active {
		t.Fatalf("second chunk: %q, %d, active=%t", text, event, gate.active)
	}
}

func TestVoiceGateAcceptsJoinedWakeCommand(t *testing.T) {
	for _, command := range []string{"енотекст", "енотек", "еноттекст"} {
		t.Run(command, func(t *testing.T) {
			gate := newVoiceGate("Енот текст", "Енот стоп")
			text, event := gate.Accept("до команды " + command + " проверяю ввод")
			if text != "проверяю ввод" || event != gateStarted || !gate.active {
				t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
			}
		})
	}
}

func TestVoiceGateStopsOnJoinedCommand(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	gate.Accept("енотекст")
	text, event := gate.Accept("это всё енотстоп больше не печатай")
	if text != "это всё" || event != gateStopped || gate.active {
		t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
	}
}

func TestVoiceGateHandlesJoinedStartAndStopTogether(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	text, event := gate.Accept("енотекст нужный текст енотстоп лишний текст")
	if text != "нужный текст" || event != gateStopped || gate.active {
		t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
	}
}

func TestVoiceGateDoesNotMatchWakeCommandInsideLongerWord(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	text, event := gate.Accept("енотекстовый редактор")
	if text != "" || event != gateUnchanged || gate.active {
		t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
	}
}

func TestVoiceGateStopsOnLatinStop(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	gate.Accept("Енот текст")
	text, event := gate.Accept("Проверяю голосовой ввод. Енот stop.")
	if text != "Проверяю голосовой ввод." || event != gateStopped || gate.active {
		t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
	}
	if text, _ := gate.Accept("после остановки"); text != "" {
		t.Fatalf("after stop: %q", text)
	}
}

func TestVoiceGateStopsOnLatinStopInStartingPhrase(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	text, event := gate.Accept("Енот текст. Сегодня хорошая погода. енот stop.")
	if text != "Сегодня хорошая погода." || event != gateStopped || gate.active {
		t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
	}
}

func TestVoiceGateIgnoresSpeechBeforeStart(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	text, event := gate.Accept("привет это обычная речь")
	if text != "" || event != gateUnchanged || gate.active {
		t.Fatalf("got %q, %d, active=%t", text, event, gate.active)
	}
}

func TestVoiceGateDictatesUntilStop(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	if text, event := gate.Accept("енот текст"); text != "" || event != gateStarted {
		t.Fatalf("start: %q, %d", text, event)
	}
	if text, event := gate.Accept("привет мир"); text != "привет мир" || event != gateUnchanged {
		t.Fatalf("dictation: %q, %d", text, event)
	}
	if text, event := gate.Accept("енот стоп"); text != "" || event != gateStopped {
		t.Fatalf("stop: %q, %d", text, event)
	}
	if text, _ := gate.Accept("после остановки"); text != "" {
		t.Fatalf("after stop: %q", text)
	}
}

func TestVoiceGateKeepsWordsAroundCommands(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	if text, event := gate.Accept("Енот, текст: привет мир"); text != "привет мир" || event != gateStarted {
		t.Fatalf("start with words: %q, %d", text, event)
	}
	if text, event := gate.Accept("до свидания енот стоп"); text != "до свидания" || event != gateStopped {
		t.Fatalf("stop with words: %q, %d", text, event)
	}
}

func TestVoiceGateAcceptsObservedRecognitionVariants(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	if _, event := gate.Accept("енот текст"); event != gateStarted {
		t.Fatalf("start event = %d", event)
	}
	if _, event := gate.Accept("енот столб"); event != gateStopped {
		t.Fatalf("stop event = %d", event)
	}
}

func TestVoiceGatePreservesPunctuation(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	if text, event := gate.Accept("Енот текст: Привет, мир!"); text != "Привет, мир!" || event != gateStarted {
		t.Fatalf("start: %q, %d", text, event)
	}
	if text, event := gate.Accept("Это всё. Енот стор"); text != "Это всё." || event != gateStopped {
		t.Fatalf("stop: %q, %d", text, event)
	}
}

func TestVoiceGateDoesNotTypeRepeatedStart(t *testing.T) {
	gate := newVoiceGate("Енот текст", "Енот стоп")
	gate.Accept("Енот текст")
	text, _ := gate.Accept("ЕНОТ ТЕКСТ. Проверяю ввод.")
	if text != "Проверяю ввод." {
		t.Fatalf("repeated start = %q", text)
	}
}
