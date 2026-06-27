package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Provider API keys are kept OUT of GlobalConfig (which the frontend reads wholesale
// and which is persisted 0644). They live in ~/.tiramisu/secrets.json (0600) and are
// resolved backend-side; the raw key never crosses to the webview.

type secretsFile struct {
	ProviderKeys map[string]string `json:"providerKeys"`
}

func (a *App) secretsPath() (string, error) {
	dir, err := a.configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "secrets.json"), nil
}

func (a *App) loadSecrets() secretsFile {
	s := secretsFile{ProviderKeys: map[string]string{}}
	path, err := a.secretsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	if s.ProviderKeys == nil {
		s.ProviderKeys = map[string]string{}
	}
	// Defensively tighten perms in case it was ever created with a looser umask.
	_ = os.Chmod(path, 0600)
	return s
}

func (a *App) saveSecrets(s secretsFile) error {
	path, err := a.secretsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// resolveProviderKey returns the stored API key for a provider, or "". UNEXPORTED so
// it is never bound to the frontend.
func (a *App) resolveProviderKey(provider string) string {
	return a.loadSecrets().ProviderKeys[provider]
}

// SetProviderKey stores (or clears, if key == "") an API key for a provider.
func (a *App) SetProviderKey(provider, key string) error {
	s := a.loadSecrets()
	if key == "" {
		delete(s.ProviderKeys, provider)
	} else {
		s.ProviderKeys[provider] = key
	}
	return a.saveSecrets(s)
}

// HasProviderKey reports whether a non-empty key is stored. Never returns the key.
func (a *App) HasProviderKey(provider string) bool {
	return a.loadSecrets().ProviderKeys[provider] != ""
}

// DeleteProviderKey removes a provider's stored key.
func (a *App) DeleteProviderKey(provider string) error {
	s := a.loadSecrets()
	delete(s.ProviderKeys, provider)
	return a.saveSecrets(s)
}
