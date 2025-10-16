package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultAccountID = "default"
	cookiesFileName  = "cookies.json"
	imagesDirName    = "images"
	dataDirName      = "accounts"
	metaFileName     = "meta.json"
)

type AccountMeta struct {
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AccountInfo struct {
	ID        string    `json:"id"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

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

func accountsRootDir() (string, error) {
	baseDir, err := baseDataDir()
	if err != nil {
		return "", err
	}

	root := filepath.Join(baseDir, dataDirName)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("failed to ensure accounts dir %s: %w", root, err)
	}
	return root, nil
}

func accountDir(accountID string) (string, error) {
	id, err := sanitizeAccountID(accountID)
	if err != nil {
		return "", err
	}

	root, err := accountsRootDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to ensure account dir %s: %w", dir, err)
	}

	return dir, nil
}

func metaPath(accountID string) (string, error) {
	dir, err := accountDir(accountID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, metaFileName), nil
}

func ensureMeta(accountID string) (*AccountMeta, error) {
	path, err := metaPath(accountID)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		meta := defaultAccountMeta()
		if err := saveAccountMeta(path, meta); err != nil {
			return nil, err
		}
		return meta, nil
	}
	if err != nil {
		return nil, err
	}

	var meta AccountMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	meta = normalizeAccountMeta(meta)
	if err := saveAccountMeta(path, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func defaultAccountMeta() *AccountMeta {
	now := time.Now()
	return &AccountMeta{
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func normalizeAccountMeta(meta AccountMeta) AccountMeta {
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = time.Now()
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = meta.CreatedAt
	}
	return meta
}

func saveAccountMeta(path string, meta *AccountMeta) error {
	meta = &AccountMeta{
		Remark:    strings.TrimSpace(meta.Remark),
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
	}
	buf, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
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
	if _, err := ensureMeta(accountID); err != nil {
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

// ListAccounts 返回所有账号信息
func ListAccounts() ([]AccountInfo, error) {
	root, err := accountsRootDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	infos := make([]AccountInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		meta, err := ensureMeta(id)
		if err != nil {
			return nil, err
		}
		infos = append(infos, AccountInfo{
			ID:        id,
			Remark:    meta.Remark,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
		})
	}

	// ensure default account present even if dir missing
	if _, err := os.Stat(filepath.Join(root, defaultAccountID)); os.IsNotExist(err) {
		if err := EnsureAccount(defaultAccountID); err != nil {
			return nil, err
		}
		meta, err := ensureMeta(defaultAccountID)
		if err != nil {
			return nil, err
		}
		infos = append(infos, AccountInfo{
			ID:        defaultAccountID,
			Remark:    meta.Remark,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})

	return infos, nil
}

// SetAccountRemark 更新账号备注
func SetAccountRemark(accountID, remark string) (*AccountInfo, error) {
	id, err := ResolveAccountID(accountID)
	if err != nil {
		return nil, err
	}

	if err := EnsureAccount(id); err != nil {
		return nil, err
	}

	path, err := metaPath(id)
	if err != nil {
		return nil, err
	}

	meta, err := ensureMeta(id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	meta.Remark = strings.TrimSpace(remark)
	meta.UpdatedAt = now

	if err := saveAccountMeta(path, meta); err != nil {
		return nil, err
	}

	return &AccountInfo{
		ID:        id,
		Remark:    meta.Remark,
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
	}, nil
}
