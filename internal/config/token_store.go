package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	TokenStorageKeyring = "keyring"
	TokenStorageFile    = "file"

	tokenStorageEnv = "COPILOT_PROXY_TOKEN_STORAGE"
	keyringService  = "copilot-proxy"
	keyringPrefix   = "keyring:"
)

// EffectiveTokenStorage selects where GitHub OAuth/PAT tokens are stored.
// Windows, macOS, and Linux desktop sessions use the system keyring. Headless
// Linux falls back to the config file, which is written with 0600 permissions.
func EffectiveTokenStorage() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(tokenStorageEnv))) {
	case TokenStorageKeyring:
		return TokenStorageKeyring
	case TokenStorageFile:
		return TokenStorageFile
	}
	if runtime.GOOS == "linux" && isHeadlessLinux() {
		return TokenStorageFile
	}
	return TokenStorageKeyring
}

func isHeadlessLinux() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	hasDisplay := os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	hasSessionBus := os.Getenv("DBUS_SESSION_BUS_ADDRESS") != ""
	return !hasDisplay || !hasSessionBus
}

func tokenRefForAccount(accountID string) string {
	return keyringPrefix + keyringUserForAccount(accountID)
}

func keyringUserForAccount(accountID string) string {
	return "github:" + accountID
}

func keyringUserFromRef(ref string) string {
	return strings.TrimPrefix(ref, keyringPrefix)
}

// StoreGitHubToken writes a GitHub token to the OS keyring and returns the
// stable reference that should be persisted in config.json.
func StoreGitHubToken(accountID, token string) (string, error) {
	if strings.TrimSpace(accountID) == "" {
		return "", errors.New("account id is required")
	}
	if token == "" {
		return "", errors.New("github token is required")
	}
	user := keyringUserForAccount(accountID)
	if err := keyring.Set(keyringService, user, token); err != nil {
		return "", err
	}
	return keyringPrefix + user, nil
}

// LoadGitHubToken resolves a keyring reference into its stored token.
func LoadGitHubToken(ref string) (string, error) {
	if !strings.HasPrefix(ref, keyringPrefix) {
		return "", fmt.Errorf("unsupported token reference %q", ref)
	}
	return keyring.Get(keyringService, keyringUserFromRef(ref))
}

// DeleteStoredGitHubToken removes a token from the OS keyring when the account
// used keyring storage. Missing entries are treated as already deleted.
func DeleteStoredGitHubToken(account AccountConfig) error {
	ref := account.GitHubTokenRef
	if ref == "" {
		if EffectiveTokenStorage() != TokenStorageKeyring {
			return nil
		}
		ref = tokenRefForAccount(account.ID)
	}
	if !strings.HasPrefix(ref, keyringPrefix) {
		return nil
	}
	if err := keyring.Delete(keyringService, keyringUserFromRef(ref)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}
