package main

import "testing"

type fakePhraseDecoder struct {
	calls   int
	samples int
}

func (f *fakePhraseDecoder) Recognize(samples []float32) (string, error) {
	f.calls++
	f.samples = len(samples)
	return "Привет, мир!", nil
}

func (f *fakePhraseDecoder) Close() {}

func TestPhraseRecognizerEndsAfterPause(t *testing.T) {
	decoder := &fakePhraseDecoder{}
	recognizer := newPhraseRecognizer(decoder)
	for i := 0; i < 3; i++ {
		if _, ready, err := recognizer.Accept(voicedChunk()); err != nil || ready {
			t.Fatalf("speech chunk: ready=%v, error=%v", ready, err)
		}
	}
	for i := 0; i < 6; i++ {
		if _, ready, err := recognizer.Accept(make([]float32, samplesPerChunk)); err != nil || ready {
			t.Fatalf("pause chunk %d: ready=%v, error=%v", i, ready, err)
		}
	}
	text, ready, err := recognizer.Accept(make([]float32, samplesPerChunk))
	if err != nil || !ready || text != "Привет, мир!" || decoder.calls != 1 {
		t.Fatalf("text=%q, ready=%v, calls=%d, error=%v", text, ready, decoder.calls, err)
	}
}

func TestPhraseRecognizerIgnoresShortNoise(t *testing.T) {
	decoder := &fakePhraseDecoder{}
	recognizer := newPhraseRecognizer(decoder)
	if _, _, err := recognizer.Accept(voicedChunk()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, _, err := recognizer.Accept(make([]float32, samplesPerChunk)); err != nil {
			t.Fatal(err)
		}
	}
	if decoder.calls != 0 {
		t.Fatalf("decoder called %d times", decoder.calls)
	}
}

func TestPhraseRecognizerEndsAtMicrophoneNoiseFloor(t *testing.T) {
	decoder := &fakePhraseDecoder{}
	recognizer := newPhraseRecognizer(decoder)
	for i := 0; i < 3; i++ {
		if _, _, err := recognizer.Accept(voicedChunk()); err != nil {
			t.Fatal(err)
		}
	}
	noise := make([]float32, samplesPerChunk)
	for i := range noise {
		noise[i] = 0.0005
	}
	var ready bool
	for i := 0; i < 10; i++ {
		_, gotReady, err := recognizer.Accept(noise)
		if err != nil {
			t.Fatal(err)
		}
		ready = ready || gotReady
	}
	if !ready || decoder.calls != 1 {
		t.Fatalf("ready=%v, calls=%d", ready, decoder.calls)
	}
}

func voicedChunk() []float32 {
	samples := make([]float32, samplesPerChunk)
	for i := range samples {
		samples[i] = 0.01
	}
	return samples
}

func TestPhraseRecognizerDiscardsAudioAcrossReconnect(t *testing.T) {
	decoder := &fakePhraseDecoder{}
	recognizer := newPhraseRecognizer(decoder)
	for i := 0; i < 3; i++ {
		if _, _, err := recognizer.Accept(voicedChunk()); err != nil {
			t.Fatal(err)
		}
	}
	recognizer.reset()
	if text, err := recognizer.Finish(); err != nil || text != "" || decoder.calls != 0 {
		t.Fatalf("text=%q calls=%d error=%v", text, decoder.calls, err)
	}
}
