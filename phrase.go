package main

import "math"

const (
	voiceThreshold = 0.002
	phrasePause    = sampleRate * 7 / 10
	phraseMinimum  = sampleRate / 4
	phraseMaximum  = sampleRate * 20
	phrasePreRoll  = sampleRate / 4
)

type phraseDecoder interface {
	Recognize([]float32) (string, error)
	Close()
}

type phraseRecognizer struct {
	decoder       phraseDecoder
	buffer        []float32
	preRoll       []float32
	voicedSamples int
	silentSamples int
}

func newPhraseRecognizer(decoder phraseDecoder) *phraseRecognizer {
	return &phraseRecognizer{decoder: decoder}
}

func (p *phraseRecognizer) Accept(samples []float32) (string, bool, error) {
	if len(samples) == 0 {
		return "", false, nil
	}
	var energy float64
	for _, sample := range samples {
		energy += float64(sample * sample)
	}
	voiced := math.Sqrt(energy/float64(len(samples))) >= voiceThreshold
	if len(p.buffer) == 0 {
		if !voiced {
			p.preRoll = append(p.preRoll, samples...)
			if len(p.preRoll) > phrasePreRoll {
				p.preRoll = p.preRoll[len(p.preRoll)-phrasePreRoll:]
			}
			return "", false, nil
		}
		p.buffer = append(p.buffer, p.preRoll...)
		p.preRoll = p.preRoll[:0]
	}
	p.buffer = append(p.buffer, samples...)
	if voiced {
		p.voicedSamples += len(samples)
		p.silentSamples = 0
	} else {
		p.silentSamples += len(samples)
	}
	if p.silentSamples < phrasePause && len(p.buffer) < phraseMaximum {
		return "", false, nil
	}
	text, err := p.flush()
	return text, text != "", err
}

func (p *phraseRecognizer) Finish() (string, error) {
	return p.flush()
}

func (p *phraseRecognizer) flush() (string, error) {
	if len(p.buffer) == 0 {
		return "", nil
	}
	samples := p.buffer
	voiced := p.voicedSamples
	p.buffer = nil
	p.voicedSamples = 0
	p.silentSamples = 0
	if voiced < phraseMinimum {
		return "", nil
	}
	return p.decoder.Recognize(samples)
}

func (p *phraseRecognizer) reset() {
	p.buffer = nil
	p.preRoll = nil
	p.voicedSamples = 0
	p.silentSamples = 0
}

func (p *phraseRecognizer) Close() { p.decoder.Close() }
