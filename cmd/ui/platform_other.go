//go:build !windows

package main

import "fmt"

type uiSingleInstanceGuard interface {
	Release()
}

type noopGuard struct{}

func (noopGuard) Release() {}

func acquireUISingleInstance() (uiSingleInstanceGuard, bool, error) {
	return noopGuard{}, true, nil
}

func focusExistingUIWindow() error {
	return fmt.Errorf("window focus is only supported on Windows")
}
