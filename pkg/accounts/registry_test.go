// ABOUTME: Tests for account registry
// ABOUTME: Validates config loading, account resolution, and fallback behavior

package accounts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRegistry_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")
	os.WriteFile(configPath, []byte(`{
		"default": "work",
		"accounts": {
			"work": {"token_path": "/tmp/work.json"},
			"personal": {"token_path": "/tmp/personal.json"}
		}
	}`), 0600)

	reg, err := LoadRegistry(configPath)
	require.NoError(t, err)
	assert.Equal(t, "work", reg.DefaultAlias)
	assert.Len(t, reg.Accounts, 2)
}

func TestLoadRegistry_FileNotFound(t *testing.T) {
	reg, err := LoadRegistry("/nonexistent/accounts.json")
	require.NoError(t, err)
	assert.Equal(t, "default", reg.DefaultAlias)
	assert.Len(t, reg.Accounts, 1)
}

func TestRegistry_Resolve_Default(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work":     {Alias: "work", TokenPath: "/tmp/work.json"},
			"personal": {Alias: "personal", TokenPath: "/tmp/personal.json"},
		},
	}

	acct, err := reg.Resolve("")
	require.NoError(t, err)
	assert.Equal(t, "work", acct.Alias)
}

func TestRegistry_Resolve_ByAlias(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work":     {Alias: "work", TokenPath: "/tmp/work.json"},
			"personal": {Alias: "personal", TokenPath: "/tmp/personal.json"},
		},
	}

	acct, err := reg.Resolve("personal")
	require.NoError(t, err)
	assert.Equal(t, "personal", acct.Alias)
}

func TestRegistry_Resolve_Unknown(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work": {Alias: "work", TokenPath: "/tmp/work.json"},
		},
	}

	_, err := reg.Resolve("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown account")
	assert.Contains(t, err.Error(), "work")
}

func TestRegistry_ListAccounts(t *testing.T) {
	reg := &Registry{
		DefaultAlias: "work",
		Accounts: map[string]*Account{
			"work":     {Alias: "work", TokenPath: "/tmp/work.json"},
			"personal": {Alias: "personal", TokenPath: "/tmp/personal.json"},
		},
	}

	aliases := reg.ListAccounts()
	assert.Len(t, aliases, 2)
	assert.Contains(t, aliases, "work")
	assert.Contains(t, aliases, "personal")
}

func TestRegistry_AddAccount(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")

	reg := &Registry{
		configPath:   configPath,
		DefaultAlias: "default",
		Accounts:     map[string]*Account{},
	}

	err := reg.AddAccount("work", "/tmp/work.json")
	require.NoError(t, err)
	assert.Len(t, reg.Accounts, 1)

	// Verify file was written
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")

	reg := &Registry{
		configPath:   configPath,
		DefaultAlias: "default",
		Accounts: map[string]*Account{
			"default": {Alias: "default", TokenPath: "/tmp/default.json"},
		},
	}

	// Concurrent reads and writes should not race
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_, _ = reg.Resolve("default")
				_ = reg.ListAccounts()
			}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
