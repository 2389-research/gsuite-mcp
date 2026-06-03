// ABOUTME: Verifies the CLI version symbol is an addressable var, not a const.
// ABOUTME: Guards the -X main.version ldflag injection target from regressing to const.

package main

import "testing"

func TestVersionIsVar(t *testing.T) {
	// Compiles only if version is an addressable var (ldflag target), not a const.
	_ = &version
}
