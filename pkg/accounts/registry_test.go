// ABOUTME: Tests for account registry
// ABOUTME: Validates config loading, account resolution, and fallback behavior

package accounts

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRegistry_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
		"default": "work",
		"accounts": {
			"work": {"token_path": "/tmp/work.json"},
			"personal": {"token_path": "/tmp/personal.json"}
		}
	}`), 0600))

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

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		alias   string
		wantErr bool
	}{
		{"work", false},
		{"personal", false},
		{"my-account", false},
		{"account_1", false},
		{"a.b.c", false},
		{"", true},                   // empty
		{"../evil", true},            // path traversal
		{"../../etc/passwd", true},   // path traversal
		{"/absolute", true},          // starts with slash
		{"hello world", true},        // spaces
		{"-leading-dash", true},      // starts with non-alphanumeric
		{strings.Repeat("a", 65), true},        // too long (65 chars, limit is 64)
	}

	for _, tt := range tests {
		err := ValidateAlias(tt.alias)
		if tt.wantErr {
			assert.Error(t, err, "alias=%q should be invalid", tt.alias)
		} else {
			assert.NoError(t, err, "alias=%q should be valid", tt.alias)
		}
	}
}

func TestRegistry_AddAccount_RejectsInvalidAlias(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")

	reg := &Registry{
		configPath:   configPath,
		DefaultAlias: "default",
		Accounts:     map[string]*Account{},
	}

	err := reg.AddAccount("../../evil", "/tmp/evil.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestRegistry_AddAccount_RejectsDuplicate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")

	reg := &Registry{
		configPath:   configPath,
		DefaultAlias: "default",
		Accounts: map[string]*Account{
			"work": {Alias: "work", TokenPath: "/tmp/work.json"},
		},
	}

	err := reg.AddAccount("work", "/tmp/work2.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestLoadRegistry_InvalidDefaultAlias(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
		"default": "nonexistent",
		"accounts": {
			"work": {"token_path": "/tmp/work.json"}
		}
	}`), 0600))

	_, err := LoadRegistry(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "default account 'nonexistent' not found")
}

func TestLoadRegistry_InvalidAliasInConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
		"default": "../../evil",
		"accounts": {
			"../../evil": {"token_path": "/tmp/evil.json"}
		}
	}`), 0600))

	_, err := LoadRegistry(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid account alias")
}

func TestAccount_ClientAccessors(t *testing.T) {
	acct := &Account{Alias: "test"}

	// Verify GetClient/SetClient work
	assert.Nil(t, acct.GetClient())

	client := &http.Client{}
	acct.SetClient(client)
	assert.Equal(t, client, acct.GetClient())

	acct.SetClient(nil)
	assert.Nil(t, acct.GetClient())
}

func TestAccount_ClientConcurrentAccess(t *testing.T) {
	acct := &Account{Alias: "test"}
	clients := make([]*http.Client, 10)
	for i := range clients {
		clients[i] = &http.Client{}
	}

	var wg sync.WaitGroup
	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(c *http.Client) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				acct.SetClient(c)
			}
		}(clients[i])
	}
	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = acct.GetClient()
			}
		}()
	}
	wg.Wait()

	// After all goroutines finish, client should be one of the set values
	got := acct.GetClient()
	assert.NotNil(t, got)
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

func TestRegistry_Default(t *testing.T) {
	t.Run("fallback registry has default alias", func(t *testing.T) {
		// LoadRegistry on a non-existent path returns a fallback with DefaultAlias == "default"
		reg, err := LoadRegistry("/nonexistent/path/accounts.json")
		require.NoError(t, err)
		assert.Equal(t, "default", reg.Default())
	})

	t.Run("loaded registry returns configured default", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "accounts.json")
		require.NoError(t, os.WriteFile(configPath, []byte(`{
			"default": "work",
			"accounts": {
				"work": {"token_path": "/tmp/work.json"}
			}
		}`), 0600))

		reg, err := LoadRegistry(configPath)
		require.NoError(t, err)
		assert.Equal(t, "work", reg.Default())
	})
}

func TestRegistry_ConcurrentAccessWithDefault(t *testing.T) {
	// This test is designed to exercise the race detector.
	// It concurrently calls Default(), ListAccounts(), Resolve(), and AddAccount()
	// to give -race meaningful coverage of all registry locking paths.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "accounts.json")

	reg, err := LoadRegistry(configPath)
	require.NoError(t, err)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			// Readers: Default, ListAccounts, Resolve
			_ = reg.Default()
			_ = reg.ListAccounts()
			_, _ = reg.Resolve("default")

			// Writer: AddAccount with a unique alias per goroutine
			alias := fmt.Sprintf("acct%d", idx)
			// ignore save() I/O errors from concurrent disk writes — this test targets the in-memory lock paths
			_ = reg.AddAccount(alias, "/tmp/"+alias+".json")

			// More reads after the write
			_ = reg.Default()
			_ = reg.ListAccounts()
		}(i)
	}

	wg.Wait()
}
