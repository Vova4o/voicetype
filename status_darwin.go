package main

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdint.h>
#include <stdlib.h>
void *voice_status_create(uintptr_t handle);
void voice_status_update(void *status, int active, int ready, const char *problem, const char *heard);
void voice_status_run(void *status);
void voice_status_stop(void);
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

type menuStatus struct{ native unsafe.Pointer }

func newMenuStatus(state *dictationState) (*menuStatus, func()) {
	handle := cgo.NewHandle(state)
	status := &menuStatus{native: C.voice_status_create(C.uintptr_t(handle))}
	return status, handle.Delete
}

func (s *menuStatus) update(active, ready bool, problem, heard string) {
	message := C.CString(problem)
	recognition := C.CString(heard)
	defer C.free(unsafe.Pointer(recognition))
	defer C.free(unsafe.Pointer(message))
	var a, r C.int
	if active {
		a = 1
	}
	if ready {
		r = 1
	}
	C.voice_status_update(s.native, a, r, message, recognition)
}

func (s *menuStatus) run() { C.voice_status_run(s.native) }
func stopMenuStatus()      { C.voice_status_stop() }

// toggleDictation handles the native menu action without moving focus to another app.
//
//export toggleDictation
func toggleDictation(handle C.uintptr_t) {
	cgo.Handle(handle).Value().(*dictationState).toggle()
}
