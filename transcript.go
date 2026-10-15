package main

import "strings"

type streamingTranscript struct {
	history   [][]string
	committed []string
}

func (s *streamingTranscript) Update(text string, final bool) string {
	words := strings.Fields(text)
	if final {
		var delta string
		if offset := alignedPrefix(words, s.committed); offset < len(words) {
			delta = strings.Join(words[offset:], " ")
		}
		s.history = nil
		s.committed = nil
		return delta
	}
	s.history = append(s.history, words)
	if len(s.history) < 3 {
		return ""
	}
	if len(s.history) > 3 {
		s.history = s.history[len(s.history)-3:]
	}
	// The last word may still grow while the speaker is pronouncing it.
	stable := len(words) - 1
	for _, previous := range s.history[:2] {
		if len(previous) < stable {
			stable = len(previous)
		}
		for i := 0; i < stable; i++ {
			if words[i] != previous[i] {
				stable = i
				break
			}
		}
	}
	if stable <= 0 {
		return ""
	}
	offset := alignedPrefix(words[:stable], s.committed)
	if offset >= stable {
		return ""
	}
	delta := strings.Join(words[offset:stable], " ")
	s.committed = append(s.committed[:0], words[:stable]...)
	return delta
}

func alignedPrefix(words, committed []string) int {
	matched := true
	for i := 0; i < min(len(words), len(committed)); i++ {
		if words[i] != committed[i] {
			matched = false
			break
		}
	}
	if matched {
		return min(len(words), len(committed))
	}
	// ASR may revise word boundaries, so align characters at word ends rather
	// than skipping the previous word count in the revised hypothesis.
	previous := []rune(strings.Join(committed, ""))
	current := []rune(strings.Join(words, ""))
	row := make([]int, len(current)+1)
	next := make([]int, len(row))
	for j := range row {
		row[j] = j
	}
	for i, a := range previous {
		next[0] = i + 1
		for j, b := range current {
			cost := 0
			if a != b {
				cost = 1
			}
			next[j+1] = min(row[j+1]+1, next[j]+1, row[j]+cost)
		}
		row, next = next, row
	}
	best, bestCost, bestLengthGap := 0, row[0], len(previous)
	end := 0
	for i, word := range words {
		end += len([]rune(word))
		gap := len(previous) - end
		if gap < 0 {
			gap = -gap
		}
		if row[end] < bestCost || (row[end] == bestCost && gap < bestLengthGap) {
			best, bestCost, bestLengthGap = i+1, row[end], gap
		}
	}
	return best
}
