package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Account struct {
	Email        string    `json:"email"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	ProjectID    string    `json:"project_id"`
	TierID       string    `json:"tier_id"`
	BlockedUntil time.Time `json:"blocked_until,omitempty"`
	Strikes      int       `json:"strikes,omitempty"`
}

type Store struct {
	mu           sync.RWMutex
	filePath     string
	lastModTime  time.Time
	ActiveEmail  string              `json:"active_email"`
	Accounts     map[string]*Account `json:"accounts"`
	currentIndex int
}

func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "agproxy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func LoadStore() (*Store, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "accounts.json")
	s := &Store{
		filePath: path,
		Accounts: make(map[string]*Account),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if fi, err := os.Stat(path); err == nil {
		s.lastModTime = fi.ModTime()
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("failed to parse accounts.json: %w", err)
	}

	if s.Accounts == nil {
		s.Accounts = make(map[string]*Account)
	}

	return s, nil
}

func (s *Store) syncFromDiskLocked() {
	fi, err := os.Stat(s.filePath)
	if err != nil {
		return
	}
	// If disk file hasn't changed since we last read it, skip
	if !fi.ModTime().After(s.lastModTime) {
		return
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	var diskStore struct {
		ActiveEmail string              `json:"active_email"`
		Accounts    map[string]*Account `json:"accounts"`
	}
	if err := json.Unmarshal(data, &diskStore); err != nil {
		return
	}

	if s.Accounts == nil {
		s.Accounts = make(map[string]*Account)
	}

	// Merge accounts: preserve local accounts and pull any new/updated accounts from disk
	for email, acc := range diskStore.Accounts {
		if localAcc, exists := s.Accounts[email]; exists {
			if acc.ExpiresAt.After(localAcc.ExpiresAt) {
				localAcc.AccessToken = acc.AccessToken
				localAcc.RefreshToken = acc.RefreshToken
				localAcc.ExpiresAt = acc.ExpiresAt
			}
			if acc.ProjectID != "" {
				localAcc.ProjectID = acc.ProjectID
			}
			if acc.TierID != "" {
				localAcc.TierID = acc.TierID
			}
		} else {
			s.Accounts[email] = acc
		}
	}

	if s.ActiveEmail == "" && diskStore.ActiveEmail != "" {
		s.ActiveEmail = diskStore.ActiveEmail
	}

	s.lastModTime = fi.ModTime()
}

func (s *Store) SyncFromDisk() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncFromDiskLocked()
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	// Always sync from disk first to merge accounts added by external CLI commands
	s.syncFromDiskLocked()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(s.filePath, data, 0600); err != nil {
		return err
	}

	if fi, err := os.Stat(s.filePath); err == nil {
		s.lastModTime = fi.ModTime()
	}
	return nil
}

func (s *Store) AddOrUpdateAccount(acc *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncFromDiskLocked()
	s.Accounts[acc.Email] = acc
	if s.ActiveEmail == "" {
		s.ActiveEmail = acc.Email
	}

	return s.saveLocked()
}

func (s *Store) RemoveAccount(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncFromDiskLocked()
	delete(s.Accounts, email)
	if s.ActiveEmail == email {
		s.ActiveEmail = ""
		for k := range s.Accounts {
			s.ActiveEmail = k
			break
		}
	}

	return s.saveLocked()
}

func (s *Store) SetActive(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncFromDiskLocked()
	if _, ok := s.Accounts[email]; !ok {
		return fmt.Errorf("account with email %s not found", email)
	}
	s.ActiveEmail = email

	return s.saveLocked()
}

func (s *Store) GetActiveAccount() *Account {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncFromDiskLocked()

	now := time.Now()
	// Check if active account is valid and not blocked
	if acc, ok := s.Accounts[s.ActiveEmail]; ok {
		if acc.BlockedUntil.IsZero() || acc.BlockedUntil.Before(now) {
			return acc
		}
	}

	// Active is blocked or missing, find another unblocked account
	for _, acc := range s.Accounts {
		if acc.BlockedUntil.IsZero() || acc.BlockedUntil.Before(now) {
			return acc
		}
	}

	// If all are blocked or none found, return whatever is active
	return s.Accounts[s.ActiveEmail]
}

func (s *Store) RotateNextAccount(failedEmail string, blockDuration time.Duration) *Account {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncFromDiskLocked()

	now := time.Now()
	if failed, ok := s.Accounts[failedEmail]; ok {
		failed.Strikes++
		failed.BlockedUntil = now.Add(blockDuration)
	}

	var candidates []*Account
	for _, acc := range s.Accounts {
		if acc.Email != failedEmail && (acc.BlockedUntil.IsZero() || acc.BlockedUntil.Before(now)) {
			candidates = append(candidates, acc)
		}
	}

	if len(candidates) > 0 {
		s.currentIndex = (s.currentIndex + 1) % len(candidates)
		next := candidates[s.currentIndex]
		s.ActiveEmail = next.Email
		_ = s.saveLocked()
		return next
	}

	_ = s.saveLocked()
	return nil
}

func (s *Store) MarkSuccess(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if acc, ok := s.Accounts[email]; ok {
		acc.Strikes = 0
		acc.BlockedUntil = time.Time{}
		_ = s.saveLocked()
	}
}
