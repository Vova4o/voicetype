package main

/*
#cgo darwin LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
#include "sherpa_c_api.h"

typedef struct {
  void *dependency, *library;
  const SherpaOnnxOnlineRecognizer *recognizer;
  const SherpaOnnxOnlineStream *stream;
  __typeof__(&SherpaOnnxDestroyOnlineRecognizer) destroy_recognizer;
  __typeof__(&SherpaOnnxCreateOnlineStream) create_stream;
  __typeof__(&SherpaOnnxDestroyOnlineStream) destroy_stream;
  __typeof__(&SherpaOnnxOnlineStreamAcceptWaveform) accept;
  __typeof__(&SherpaOnnxIsOnlineStreamReady) ready;
  __typeof__(&SherpaOnnxDecodeOnlineStream) decode;
  __typeof__(&SherpaOnnxGetOnlineStreamResult) result;
  __typeof__(&SherpaOnnxDestroyOnlineRecognizerResult) destroy_result;
  __typeof__(&SherpaOnnxOnlineStreamIsEndpoint) endpoint;
  __typeof__(&SherpaOnnxOnlineStreamReset) reset;
  __typeof__(&SherpaOnnxOnlineStreamInputFinished) finish;
} VoskEngine;

static const char *vosk_load(VoskEngine *e, const char *library, const char *dependency,
  const char *encoder, const char *decoder, const char *joiner, const char *tokens, int threads) {
  memset(e, 0, sizeof(*e));
  e->dependency = dlopen(dependency, RTLD_NOW | RTLD_GLOBAL);
  if (!e->dependency) return dlerror();
  e->library = dlopen(library, RTLD_NOW | RTLD_LOCAL);
  if (!e->library) return dlerror();
#define VOSK_LOAD(field, symbol) do { e->field = (void *)dlsym(e->library, symbol); if (!e->field) return dlerror(); } while (0)
  __typeof__(&SherpaOnnxCreateOnlineRecognizer) create = (void *)dlsym(e->library, "SherpaOnnxCreateOnlineRecognizer");
  if (!create) return dlerror();
  VOSK_LOAD(destroy_recognizer, "SherpaOnnxDestroyOnlineRecognizer");
  VOSK_LOAD(create_stream, "SherpaOnnxCreateOnlineStream");
  VOSK_LOAD(destroy_stream, "SherpaOnnxDestroyOnlineStream");
  VOSK_LOAD(accept, "SherpaOnnxOnlineStreamAcceptWaveform");
  VOSK_LOAD(ready, "SherpaOnnxIsOnlineStreamReady");
  VOSK_LOAD(decode, "SherpaOnnxDecodeOnlineStream");
  VOSK_LOAD(result, "SherpaOnnxGetOnlineStreamResult");
  VOSK_LOAD(destroy_result, "SherpaOnnxDestroyOnlineRecognizerResult");
  VOSK_LOAD(endpoint, "SherpaOnnxOnlineStreamIsEndpoint");
  VOSK_LOAD(reset, "SherpaOnnxOnlineStreamReset");
  VOSK_LOAD(finish, "SherpaOnnxOnlineStreamInputFinished");
#undef VOSK_LOAD
  SherpaOnnxOnlineRecognizerConfig config = {0};
  config.feat_config.sample_rate = 16000;
  config.feat_config.feature_dim = 80;
  config.model_config.transducer.encoder = encoder;
  config.model_config.transducer.decoder = decoder;
  config.model_config.transducer.joiner = joiner;
  config.model_config.tokens = tokens;
  config.model_config.num_threads = threads;
  config.model_config.provider = "cpu";
  config.decoding_method = "greedy_search";
  config.enable_endpoint = 1;
  config.rule1_min_trailing_silence = 2.4;
  config.rule2_min_trailing_silence = 1.0;
  config.rule3_min_utterance_length = 20.0;
  e->recognizer = create(&config);
  if (!e->recognizer) return "не удалось создать распознаватель vosk";
  e->stream = e->create_stream(e->recognizer);
  if (!e->stream) return "не удалось создать поток vosk";
  return NULL;
}

static int vosk_decode(VoskEngine *e) {
  int count = 0;
  while (e->ready(e->recognizer, e->stream)) {
    e->decode(e->recognizer, e->stream);
    ++count;
  }
  return count;
}

static int vosk_accept(VoskEngine *e, const float *samples, int n) {
  e->accept(e->stream, 16000, samples, n);
  if (!vosk_decode(e)) return -1;
  return e->endpoint(e->recognizer, e->stream);
}

static void vosk_reset(VoskEngine *e) { e->reset(e->recognizer, e->stream); }
static void vosk_finish(VoskEngine *e) {
  float padding[12800] = {0};
  e->accept(e->stream, 16000, padding, 12800);
  e->finish(e->stream);
  vosk_decode(e);
}

static char *vosk_text(VoskEngine *e) {
  const SherpaOnnxOnlineRecognizerResult *r = e->result(e->recognizer, e->stream);
  char *text = r && r->text ? strdup(r->text) : NULL;
  if (r) e->destroy_result(r);
  return text;
}

static void vosk_close(VoskEngine *e) {
  if (e->stream) e->destroy_stream(e->stream);
  if (e->recognizer) e->destroy_recognizer(e->recognizer);
  if (e->library) dlclose(e->library);
  if (e->dependency) dlclose(e->dependency);
  memset(e, 0, sizeof(*e));
}
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"
)

type vosk struct {
	engine     C.VoskEngine
	finished   bool
	transcript streamingTranscript
}

func newVosk(library string, files modelFiles, threads int) (*vosk, error) {
	values := []string{library, filepath.Join(filepath.Dir(library), "libonnxruntime.dylib"), files.encoder, files.decoder, files.joiner, files.tokens}
	cstrings := make([]*C.char, len(values))
	for i, value := range values {
		cstrings[i] = C.CString(value)
		defer C.free(unsafe.Pointer(cstrings[i]))
	}
	v := &vosk{}
	if problem := C.vosk_load(&v.engine, cstrings[0], cstrings[1], cstrings[2], cstrings[3], cstrings[4], cstrings[5], C.int(threads)); problem != nil {
		message := C.GoString(problem)
		v.Close()
		return nil, fmt.Errorf("загрузить vosk: %s", message)
	}
	return v, nil
}

func (v *vosk) Accept(samples []float32) (string, bool, error) {
	if v.finished || v.engine.stream == nil {
		return "", false, fmt.Errorf("поток vosk уже завершён")
	}
	if len(samples) == 0 {
		return "", false, nil
	}
	status := C.vosk_accept(&v.engine, (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	if status < 0 {
		return "", false, nil
	}
	text, err := v.text()
	if err != nil {
		return "", false, err
	}
	final := status != 0
	delta := v.transcript.Update(text, final)
	if final {
		C.vosk_reset(&v.engine)
	}
	return delta, delta != "", nil
}

func (v *vosk) Finish() (string, error) {
	if v.finished {
		return "", nil
	}
	if v.engine.stream == nil {
		return "", fmt.Errorf("поток vosk закрыт")
	}
	v.finished = true
	C.vosk_finish(&v.engine)
	text, err := v.text()
	if err != nil {
		return "", err
	}
	return v.transcript.Update(text, true), nil
}

func (v *vosk) text() (string, error) {
	text := C.vosk_text(&v.engine)
	if text == nil {
		return "", fmt.Errorf("не удалось получить результат vosk")
	}
	defer C.free(unsafe.Pointer(text))
	return strings.TrimSpace(C.GoString(text)), nil
}

func (v *vosk) Close() {
	C.vosk_close(&v.engine)
}
