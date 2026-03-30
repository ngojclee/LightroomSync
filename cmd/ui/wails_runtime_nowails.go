//go:build !wails

package main

import "fmt"

func launchWailsRuntime(_ string) error {
	return fmt.Errorf("wails runtime is not included in this build; rebuild UI with -UIRuntime wails")
}
