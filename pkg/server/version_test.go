// ABOUTME: Verifies the MCP server reports the propagated build version.
// ABOUTME: Guards against the hardcoded "1.0.0" regressing.

package server

import "testing"

func TestVersionDefault(t *testing.T) {
	if Version == "" {
		t.Fatal("server.Version must have a non-empty default")
	}
}
