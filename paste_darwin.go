package main

/*
#cgo darwin LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

static int post_command_v(void) {
    if (!AXIsProcessTrusted()) {
        return 1;
    }
    CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, true);
    CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)9, false);
    if (down == NULL || up == NULL) {
        if (down != NULL) CFRelease(down);
        if (up != NULL) CFRelease(up);
        return 2;
    }
    CGEventSetFlags(down, kCGEventFlagMaskCommand);
    CGEventSetFlags(up, kCGEventFlagMaskCommand);
    CGEventPost(kCGHIDEventTap, down);
    CGEventPost(kCGHIDEventTap, up);
    CFRelease(down);
    CFRelease(up);
    return 0;
}
*/
import "C"

import "errors"

func pressCommandV() error {
	switch C.post_command_v() {
	case 0:
		return nil
	case 1:
		return errors.New("для voicetype не разрешено управление компьютером в настройках macOS")
	default:
		return errors.New("не удалось создать событие клавиатуры")
	}
}

func accessibilityTrusted() bool {
	return C.AXIsProcessTrusted() != 0
}
