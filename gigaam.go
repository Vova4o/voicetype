package main

/*
#cgo darwin LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdlib.h>
#include <string.h>
#include "sherpa_c_api.h"

typedef const SherpaOnnxOfflineRecognizer *(*giga_create_recognizer_fn)(const SherpaOnnxOfflineRecognizerConfig *);
typedef void (*giga_destroy_recognizer_fn)(const SherpaOnnxOfflineRecognizer *);
typedef const SherpaOnnxOfflineStream *(*giga_create_stream_fn)(const SherpaOnnxOfflineRecognizer *);
typedef void (*giga_destroy_stream_fn)(const SherpaOnnxOfflineStream *);
typedef void (*giga_accept_fn)(const SherpaOnnxOfflineStream *, int32_t, const float *, int32_t);
typedef void (*giga_decode_fn)(const SherpaOnnxOfflineRecognizer *, const SherpaOnnxOfflineStream *);
typedef const SherpaOnnxOfflineRecognizerResult *(*giga_result_fn)(const SherpaOnnxOfflineStream *);
typedef void (*giga_destroy_result_fn)(const SherpaOnnxOfflineRecognizerResult *);

typedef struct {
  void *dependency, *library;
  const SherpaOnnxOfflineRecognizer *recognizer;
  giga_destroy_recognizer_fn destroy_recognizer;
  giga_create_stream_fn create_stream;
  giga_destroy_stream_fn destroy_stream;
  giga_accept_fn accept;
  giga_decode_fn decode;
  giga_result_fn result;
  giga_destroy_result_fn destroy_result;
} GigaEngine;

static const char *giga_load(GigaEngine *e, const char *library, const char *dependency,
  const char *encoder, const char *decoder, const char *joiner, const char *tokens, int threads) {
  memset(e, 0, sizeof(*e));
  e->dependency = dlopen(dependency, RTLD_NOW | RTLD_GLOBAL);
  if (!e->dependency) return dlerror();
  e->library = dlopen(library, RTLD_NOW | RTLD_LOCAL);
  if (!e->library) return dlerror();
#define GIGA_LOAD(field, symbol) do { e->field = (void *)dlsym(e->library, symbol); if (!e->field) return dlerror(); } while (0)
  giga_create_recognizer_fn create = (void *)dlsym(e->library, "SherpaOnnxCreateOfflineRecognizer");
  if (!create) return dlerror();
  GIGA_LOAD(destroy_recognizer, "SherpaOnnxDestroyOfflineRecognizer");
  GIGA_LOAD(create_stream, "SherpaOnnxCreateOfflineStream");
  GIGA_LOAD(destroy_stream, "SherpaOnnxDestroyOfflineStream");
  GIGA_LOAD(accept, "SherpaOnnxAcceptWaveformOffline");
  GIGA_LOAD(decode, "SherpaOnnxDecodeOfflineStream");
  GIGA_LOAD(result, "SherpaOnnxGetOfflineStreamResult");
  GIGA_LOAD(destroy_result, "SherpaOnnxDestroyOfflineRecognizerResult");
#undef GIGA_LOAD
  SherpaOnnxOfflineRecognizerConfig config = {0};
  config.feat_config.sample_rate = 16000;
  config.feat_config.feature_dim = 64;
  config.model_config.transducer.encoder = encoder;
  config.model_config.transducer.decoder = decoder;
  config.model_config.transducer.joiner = joiner;
  config.model_config.tokens = tokens;
  config.model_config.num_threads = threads;
  config.model_config.provider = "cpu";
  config.model_config.model_type = "nemo_transducer";
  config.decoding_method = "greedy_search";
  e->recognizer = create(&config);
  if (!e->recognizer) return "не удалось создать распознаватель gigaam";
  return NULL;
}

static char *giga_recognize(GigaEngine *e, const float *samples, int n) {
  const SherpaOnnxOfflineStream *stream = e->create_stream(e->recognizer);
  if (!stream) return NULL;
  e->accept(stream, 16000, samples, n);
  e->decode(e->recognizer, stream);
  const SherpaOnnxOfflineRecognizerResult *result = e->result(stream);
  char *text = result && result->text ? strdup(result->text) : NULL;
  if (result) e->destroy_result(result);
  e->destroy_stream(stream);
  return text;
}

static void giga_close(GigaEngine *e) {
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

type gigaAM struct{ engine C.GigaEngine }

func newGigaAM(library string, files modelFiles, threads int) (*gigaAM, error) {
	values := []string{library, filepath.Join(filepath.Dir(library), "libonnxruntime.dylib"), files.encoder, files.decoder, files.joiner, files.tokens}
	cstrings := make([]*C.char, len(values))
	for i, value := range values {
		cstrings[i] = C.CString(value)
		defer C.free(unsafe.Pointer(cstrings[i]))
	}
	engine := &gigaAM{}
	problem := C.giga_load(&engine.engine, cstrings[0], cstrings[1], cstrings[2], cstrings[3], cstrings[4], cstrings[5], C.int(threads))
	if problem != nil {
		message := C.GoString(problem)
		engine.Close()
		return nil, fmt.Errorf("загрузить gigaam: %s", message)
	}
	return engine, nil
}

func (g *gigaAM) Recognize(samples []float32) (string, error) {
	if len(samples) == 0 {
		return "", nil
	}
	value := C.giga_recognize(&g.engine, (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	if value == nil {
		return "", fmt.Errorf("не удалось получить результат gigaam")
	}
	defer C.free(unsafe.Pointer(value))
	return strings.TrimSpace(C.GoString(value)), nil
}

func (g *gigaAM) Close() { C.giga_close(&g.engine) }
