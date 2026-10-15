package main

import (
	"strings"
	"unicode"
)

type gateEvent uint8

const (
	gateUnchanged gateEvent = iota
	gateStarted
	gateStopped
)

type voiceGate struct {
	start   []string
	stop    []string
	active  bool
	pending string
}

type wordSpan struct {
	word       string
	start, end int
}

func newVoiceGate(start, stop string) *voiceGate {
	return &voiceGate{start: commandForms(start), stop: commandForms(stop)}
}

func (g *voiceGate) Accept(text string) (string, gateEvent) {
	text = g.pending + text
	g.pending = ""
	spans := splitWords(text)
	for i := max(0, len(spans)-2); i < len(spans); i++ {
		var suffix string
		for _, span := range spans[i:] {
			suffix += span.word
		}
		if isCommandPrefix(suffix, g.start) || isCommandPrefix(suffix, g.stop) {
			g.pending = text[spans[i].start:] + " "
			text = strings.TrimSpace(text[:spans[i].start])
			break
		}
	}
	payload, event := g.acceptText(text)
	if event == gateStopped {
		g.pending = ""
	}
	return payload, event
}

func (g *voiceGate) acceptText(text string) (string, gateEvent) {
	spans := splitWords(text)
	parts := make([]string, len(spans))
	for i := range spans {
		parts[i] = spans[i].word
	}
	if !g.active {
		index, count := findPhrase(parts, g.start)
		if index < 0 {
			return "", gateUnchanged
		}
		g.active = true
		payloadStart := spans[index+count-1].end
		if stopIndex, _ := findPhrase(parts[index+count:], g.stop); stopIndex >= 0 {
			g.active = false
			stopIndex += index + count
			return strings.TrimSpace(trimCommandPrefix(text[payloadStart:spans[stopIndex].start])), gateStopped
		}
		return trimCommandPrefix(text[payloadStart:]), gateStarted
	}
	if index, count := findPhrase(parts, g.start); index == 0 {
		return g.acceptText(trimCommandPrefix(text[spans[count-1].end:]))
	}
	if index, _ := findPhrase(parts, g.stop); index >= 0 {
		g.active = false
		return strings.TrimSpace(text[:spans[index].start]), gateStopped
	}
	return text, gateUnchanged
}

func trimCommandPrefix(text string) string {
	return strings.TrimLeft(text, " \t\n,.:;—-")
}

func findPhrase(parts, forms []string) (int, int) {
	for i := range parts {
		var candidate string
		for j := i; j < min(i+3, len(parts)); j++ {
			candidate += parts[j]
			for _, form := range forms {
				if candidate == form {
					return i, j - i + 1
				}
			}
		}
	}
	return -1, 0
}

func commandForms(command string) []string {
	parts := words(command)
	if len(parts) != 2 || parts[0] != "енот" {
		return []string{strings.Join(parts, "")}
	}
	actions := []string{parts[1]}
	switch parts[1] {
	case "текст":
		actions = []string{"текст", "текс", "тек"}
	case "стоп":
		actions = []string{"стоп", "столб", "стор", "stop"}
	}
	var forms []string
	for _, name := range []string{"енот", "инот", "енод", "инод"} {
		for _, action := range actions {
			forms = append(forms, name+action)
			if strings.HasSuffix(name, "т") && strings.HasPrefix(action, "т") {
				forms = append(forms, name+strings.TrimPrefix(action, "т"))
			}
		}
	}
	return forms
}

func isCommandPrefix(candidate string, forms []string) bool {
	if candidate == "" {
		return false
	}
	prefix := false
	for _, form := range forms {
		if candidate == form {
			return false
		}
		prefix = prefix || strings.HasPrefix(form, candidate)
	}
	return prefix
}

func words(text string) []string {
	spans := splitWords(text)
	parts := make([]string, len(spans))
	for i := range spans {
		parts[i] = spans[i].word
	}
	return parts
}

func splitWords(text string) []wordSpan {
	var spans []wordSpan
	start := -1
	for index, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if start < 0 {
				start = index
			}
			continue
		}
		if start >= 0 {
			spans = append(spans, wordSpan{word: strings.ToLower(strings.ReplaceAll(text[start:index], "ё", "е")), start: start, end: index})
			start = -1
		}
	}
	if start >= 0 {
		spans = append(spans, wordSpan{word: strings.ToLower(strings.ReplaceAll(text[start:], "ё", "е")), start: start, end: len(text)})
	}
	return spans
}
