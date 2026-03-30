package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// credentialFileJSON is the on-disk JSON structure for Claude credentials.
type credentialFileJSON struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
}

// readCredentialFile loads and parses a Claude credential file, using an
// advisory shared lock to avoid races with Claude Code.
func readCredentialFile(path string) (*oauthToken, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("~/.claude/.credentials.json not found. Install Claude Code and run `claude login`.")
		}
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("permission denied reading %s: %w", path, err)
		}
		return nil, fmt.Errorf("failed to open credential file %s: %w", path, err)
	}
	defer file.Close()

	// Acquire a shared advisory lock on the credential file itself. The write path
	// takes an exclusive lock on a sidecar .lock file before atomically renaming a
	// fully-written temp file into place, so readers still see either the old file
	// contents or the new file contents without observing a partial write.
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("failed to lock credential file %s: %w", path, err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	var cred credentialFileJSON
	if err := json.NewDecoder(file).Decode(&cred); err != nil {
		return nil, fmt.Errorf("Failed to parse Claude credentials at ~/.claude/.credentials.json: %s", err)
	}

	if cred.AccessToken == "" {
		return nil, fmt.Errorf("Claude credentials file missing accessToken field")
	}
	if cred.RefreshToken == "" {
		return nil, fmt.Errorf("Claude credentials file missing refreshToken field")
	}

	var expiresAt time.Time
	if cred.ExpiresAt == "" {
		return nil, fmt.Errorf("Claude credentials file has invalid expiresAt: empty value")
	}
	expiresAt, err = time.Parse(time.RFC3339, cred.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("Claude credentials file has invalid expiresAt: %s", err)
	}

	return &oauthToken{
		AccessToken:  cred.AccessToken,
		RefreshToken: cred.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// writeCredentialFile persists the cached OAuth token to disk using atomic
// file replacement with an exclusive advisory lock. The caller must hold
// cm.mu.Lock().
func (cm *CredentialManager) writeCredentialFile() error {
	cred := credentialFileJSON{
		AccessToken:  cm.cached.AccessToken,
		RefreshToken: cm.cached.RefreshToken,
		ExpiresAt:    cm.cached.ExpiresAt.Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(cred, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(cm.credPath)

	// Create temp file in the same directory as the credential file.
	tmpFile, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to write temporary credential file: %w", err)
	}
	tmpPath := tmpFile.Name()

	renamed := false
	defer func() {
		if !renamed {
			os.Remove(tmpPath)
		}
	}()

	// Set permissions to 0600.
	if err := os.Chmod(tmpPath, 0600); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temporary credential file: %w", err)
	}

	// Acquire exclusive lock on the credential file (or lock file).
	lockPath := cm.credPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temporary credential file: %w", err)
	}
	defer lockFile.Close()

	// Retry loop: try to acquire exclusive non-blocking lock at 100ms intervals
	// for up to 5 seconds.
	lockAcquired := false
	for i := 0; i < 50; i++ {
		err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			lockAcquired = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !lockAcquired {
		tmpFile.Close()
		slog.Warn("failed to acquire lock on credential file after 5s, skipping write-back")
		return nil
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Write JSON bytes to temp file.
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temporary credential file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temporary credential file: %w", err)
	}
	tmpFile.Close()

	// Atomically replace the credential file.
	if err := os.Rename(tmpPath, cm.credPath); err != nil {
		return fmt.Errorf("failed to update credential file: %w", err)
	}
	renamed = true

	return nil
}
