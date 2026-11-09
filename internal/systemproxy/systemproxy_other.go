//go:build !windows

package systemproxy

import "errors"

func Current(string) Status { return Status{} }

func Enable(string, string) error {
	return errors.New("the system proxy switch is available on Windows; use the system's network settings here")
}

func Disable(string) error { return nil }
