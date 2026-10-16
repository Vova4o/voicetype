package main

/*
#cgo LDFLAGS: -framework CoreAudio
#include <CoreAudio/CoreAudio.h>

static OSStatus voice_default_input(AudioDeviceID *device) {
 AudioObjectPropertyAddress property = {
  kAudioHardwarePropertyDefaultInputDevice,
  kAudioObjectPropertyScopeGlobal,
  kAudioObjectPropertyElementMain
 };
 UInt32 size = sizeof(*device);
 return AudioObjectGetPropertyData(kAudioObjectSystemObject, &property, 0, NULL, &size, device);
}
*/
import "C"

import "fmt"

func defaultInputDeviceID() (uint32, error) {
	var device C.AudioDeviceID
	if status := C.voice_default_input(&device); status != 0 {
		return 0, fmt.Errorf("получить системный микрофон: coreaudio status %d", int32(status))
	}
	if device == C.kAudioObjectUnknown {
		return 0, fmt.Errorf("системный микрофон недоступен")
	}
	return uint32(device), nil
}
