package accounts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultAccountID = "default"
	cookiesFileName  = "cookies.json"
	imagesDirName    = "images"
	dataDirName      = "accounts"
)

var accountIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// sanitizeAccountID ensures the provided account identifier is safe for filesystem use.
func sanitizeAccountID(accountID string) (string, error) {
	trimmed := strings.TrimSpace(accountID)
	if trimmed == "" {
		return defaultAccountID, nil
	}

	if !accountIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid account_id: %s", accountID)
	}

	return trimmed, nil
}

// baseDataDir returns the root directory for account data, creating it if necessary.
func baseDataDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("XHS_MCP_DATA_DIR")); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("failed to ensure data dir %s: %w", dir, err)
		}
		return dir, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine working directory: %w", err)
	}

	dir := filepath.Join(cwd, "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to ensure data dir %s: %w", dir, err)
	}

	return dir, nil
}

func accountDir(accountID string) (string, error) {
	id, err := sanitizeAccountID(accountID)
	if err != nil {
		return "", err
	}

	baseDir, err := baseDataDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(baseDir, dataDirName, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to ensure account dir %s: %w", dir, err)
	}

	return dir, nil
}

// CookiesPath returns the cookies file path for the given account, ensuring directories exist.
func CookiesPath(accountID string) (string, error) {
	dir, err := accountDir(accountID)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, cookiesFileName), nil
}

// ImagesDir returns the per-account directory for downloaded images, ensuring it exists.
func ImagesDir(accountID string) (string, error) {
	dir, err := accountDir(accountID)
	if err != nil {
		return "", err
	}

	imagesDir := filepath.Join(dir, imagesDirName)
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to ensure images dir %s: %w", imagesDir, err)
	}

	return imagesDir, nil
}

// ValidateAccountID checks whether an account identifier is acceptable without creating resources.
func ValidateAccountID(accountID string) error {
	_, err := sanitizeAccountID(accountID)
	if err != nil {
		return err
	}
	return nil
}

// EnsureAccount prepares filesystem resources for the given account.
func EnsureAccount(accountID string) error {
	if _, err := CookiesPath(accountID); err != nil {
		return err
	}
	if _, err := ImagesDir(accountID); err != nil {
		return err
	}
	return nil
}

// DefaultAccountID exposes the default identifier for callers that want an explicit value.
func DefaultAccountID() string {
	return defaultAccountID
}

// ResolveAccountID sanitizes the provided ID and returns the resolved version used internally.
func ResolveAccountID(accountID string) (string, error) {
	id, err := sanitizeAccountID(accountID)
	if err != nil {
		return "", err
	}
	return id, nil
}

// IsDefaultAccount reports whether the provided account ID maps to the default account.
func IsDefaultAccount(accountID string) bool {
	resolved, err := sanitizeAccountID(accountID)
	if err != nil {
		return false
	}
	return resolved == defaultAccountID
}

// ErrMissingAccountID is returned when the account identifier is empty and callers require it.
var ErrMissingAccountID = errors.New("account_id is required")
