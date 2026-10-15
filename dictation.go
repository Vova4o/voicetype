package main

import (
	"strings"
	"sync"
)

type dictationState struct {
	mu      sync.Mutex
	gate    *voiceGate
	ready   bool
	problem string
	heard   string
	report  func(active, ready bool, problem, heard string)
}

func newDictationState(report func(bool, bool, string, string)) *dictationState {
	return &dictationState{gate: newVoiceGate("Енот текст", "Енот стоп"), report: report}
}

func (s *dictationState) publish() {
	if s.report != nil {
		s.report(s.gate.active, s.ready, s.problem, s.heard)
	}
}

func (s *dictationState) setReady() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	s.publish()
}

func (s *dictationState) toggle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready {
		return
	}
	s.gate.active = !s.gate.active
	s.gate.pending = ""
	s.problem = ""
	s.publish()
}

func (s *dictationState) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.problem = err.Error()
	s.publish()
}

func (s *dictationState) pasted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.problem != "" {
		s.problem = ""
		s.publish()
	}
}

func (s *dictationState) unavailable(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	s.gate.active = false
	s.problem = err.Error()
	s.publish()
}

func (s *dictationState) accept(text string) (string, gateEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	heard := []rune(strings.TrimSpace(text))
	if len(heard) > 80 {
		heard = append(heard[:80], []rune("…")...)
	}
	s.heard = string(heard)
	payload, event := s.gate.Accept(text)
	if event != gateUnchanged {
		s.problem = ""
	}
	s.publish()
	return payload, event
}
