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

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("failed to parse accounts.json: %w", err)
	}

	if s.Accounts == nil {
		s.Accounts = make(map[string]*Account)
	}

	return s, nil
}

func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600)
}

func (s *Store) AddOrUpdateAccount(acc *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Accounts[acc.Email] = acc
	if s.ActiveEmail == "" {
		s.ActiveEmail = acc.Email
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

func (s *Store) RemoveAccount(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Accounts, email)
	if s.ActiveEmail == email {
		s.ActiveEmail = ""
		for k := range s.Accounts {
			s.ActiveEmail = k
			break
		}
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

func (s *Store) SetActive(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Accounts[email]; !ok {
		return fmt.Errorf("account with email %s not found", email)
	}
	s.ActiveEmail = email

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

func (s *Store) GetActiveAccount() *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

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

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}
