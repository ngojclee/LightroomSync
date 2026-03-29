//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	winplatform "github.com/ngojclee/lightroom-sync/internal/platform/windows"
)

type uiSingleInstanceGuard interface {
	Release()
}

var (
	user32ProcFindWindowW         = syscall.NewLazyDLL("user32.dll").NewProc("FindWindowW")
	user32ProcShowWindow          = syscall.NewLazyDLL("user32.dll").NewProc("ShowWindow")
	user32ProcSetForegroundWindow = syscall.NewLazyDLL("user32.dll").NewProc("SetForegroundWindow")
)

const (
	swRestore = 9
)

func acquireUISingleInstance() (uiSingleInstanceGuard, bool, error) {
	guard := winplatform.NewSingleInstance("LightroomSyncUI_Mutex")
	acquired, err := guard.TryAcquire()
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}
	return guard, true, nil
}

func focusExistingUIWindow() error {
	titlePtr, err := syscall.UTF16PtrFromString(uiHarnessWindowTitle)
	if err != nil {
		return fmt.Errorf("encode window title: %w", err)
	}

	hwnd, _, _ := user32ProcFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return fmt.Errorf("window with title %q not found", uiHarnessWindowTitle)
	}

	_, _, _ = user32ProcShowWindow.Call(hwnd, swRestore)
	ret, _, callErr := user32ProcSetForegroundWindow.Call(hwnd)
	if ret == 0 && callErr != syscall.Errno(0) {
		return fmt.Errorf("set foreground window: %w", callErr)
	}
	return nil
}
