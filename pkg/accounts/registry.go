// ABOUTME: Account registry for multi-account Google OAuth support
// ABOUTME: Loads account config, resolves aliases, manages token paths

package accounts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Account represents a single Google account with its token path
type Account struct {
	Alias     string       `json:"-"`
	TokenPath string       `json:"token_path"`
	Client    *http.Client `json:"-"`
}

// Registry manages multiple Google accounts
type Registry struct {
	configPath   string
	DefaultAlias string              `json:"default"`
	Accounts     map[string]*Account `json:"accounts"`
	mu           sync.RWMutex        // protects Accounts map
}

// configFile is the JSON structure on disk
type configFile struct {
	Default  string                   `json:"default"`
	Accounts map[string]*accountEntry `json:"accounts"`
}

type accountEntry struct {
	TokenPath string `json:"token_path"`
}

// LoadRegistry loads accounts from a config file, or creates a single-account
// fallback registry if the file doesn't exist
func LoadRegistry(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{
				configPath:   path,
				DefaultAlias: "default",
				Accounts: map[string]*Account{
					"default": {Alias: "default"},
				},
			}, nil
		}
		return nil, fmt.Errorf("failed to read accounts config: %w", err)
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse accounts config: %w", err)
	}

	reg := &Registry{
		configPath:   path,
		DefaultAlias: cfg.Default,
		Accounts:     make(map[string]*Account, len(cfg.Accounts)),
	}

	for alias, entry := range cfg.Accounts {
		reg.Accounts[alias] = &Account{
			Alias:     alias,
			TokenPath: entry.TokenPath,
		}
	}

	return reg, nil
}

// Resolve returns the account for the given alias, or the default if alias is empty
func (r *Registry) Resolve(alias string) (*Account, error) {
	if alias == "" {
		alias = r.DefaultAlias
	}

	r.mu.RLock()
	acct, ok := r.Accounts[alias]
	r.mu.RUnlock()
	if !ok {
		available := r.ListAccounts()
		return nil, fmt.Errorf("unknown account '%s'. Available accounts: %s", alias, strings.Join(available, ", "))
	}
	return acct, nil
}

// GetDefault returns the default account
func (r *Registry) GetDefault() (*Account, error) {
	return r.Resolve("")
}

// ListAccounts returns all account aliases sorted alphabetically
func (r *Registry) ListAccounts() []string {
	r.mu.RLock()
	aliases := make([]string, 0, len(r.Accounts))
	for alias := range r.Accounts {
		aliases = append(aliases, alias)
	}
	r.mu.RUnlock()
	sort.Strings(aliases)
	return aliases
}

// AddAccount adds a new account to the registry and persists the config
func (r *Registry) AddAccount(alias, tokenPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Accounts[alias] = &Account{
		Alias:     alias,
		TokenPath: tokenPath,
	}
	return r.save()
}

// save persists the registry to disk
func (r *Registry) save() error {
	cfg := configFile{
		Default:  r.DefaultAlias,
		Accounts: make(map[string]*accountEntry, len(r.Accounts)),
	}
	for alias, acct := range r.Accounts {
		cfg.Accounts[alias] = &accountEntry{
			TokenPath: acct.TokenPath,
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal accounts config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(r.configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(r.configPath, data, 0600)
}
