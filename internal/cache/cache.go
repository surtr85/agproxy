package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// CacheEntry represents a cached completion stream or response
type CacheEntry struct {
	Key          string
	Chunks       [][]byte // Recorded SSE chunks
	FullResponse []byte   // Non-streaming full response
	PromptTokens int
	OutputTokens int
	TotalTokens  int
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

// Stats holds telemetry for cache hits and token savings
type Stats struct {
	Hits           int64 `json:"hits"`
	Misses         int64 `json:"misses"`
	TokensSaved    int64 `json:"tokens_saved"`
	EntriesCount   int   `json:"entries_count"`
	EstimatedCache int   `json:"estimated_cache_percent"`
}

// MemoryCache is a high-performance in-memory cache with exact prefix & prompt hashing
type MemoryCache struct {
	mu           sync.RWMutex
	entries      map[string]*CacheEntry
	ttl          time.Duration
	hits         int64
	misses       int64
	tokensSaved  int64
}

var (
	defaultCache *MemoryCache
	once         sync.Once
)

// GetInstance returns the singleton high-performance gateway cache
func GetInstance() *MemoryCache {
	once.Do(func() {
		defaultCache = NewMemoryCache(30 * time.Minute)
	})
	return defaultCache
}

func NewMemoryCache(ttl time.Duration) *MemoryCache {
	c := &MemoryCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
	go c.cleanupLoop()
	return c
}

func (c *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for k, v := range c.entries {
			if now.After(v.ExpiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

// ComputeCacheKey computes a deterministic SHA256 digest of request inputs
func ComputeCacheKey(model string, messages any, tools any) string {
	hasher := sha256.New()
	hasher.Write([]byte(model))
	hasher.Write([]byte("|"))

	if mBytes, err := json.Marshal(messages); err == nil {
		hasher.Write(mBytes)
	}
	hasher.Write([]byte("|"))

	if tBytes, err := json.Marshal(tools); err == nil {
		hasher.Write(tBytes)
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// Get attempts to retrieve a valid cache entry
func (c *MemoryCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	entry, found := c.entries[key]
	c.mu.RUnlock()

	if !found || time.Now().After(entry.ExpiresAt) {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}

	atomic.AddInt64(&c.hits, 1)
	atomic.AddInt64(&c.tokensSaved, int64(entry.PromptTokens))
	return entry, true
}

// Set stores a response in cache
func (c *MemoryCache) Set(key string, chunks [][]byte, fullResp []byte, promptTokens, outputTokens, totalTokens int) {
	now := time.Now()
	entry := &CacheEntry{
		Key:          key,
		Chunks:       chunks,
		FullResponse: fullResp,
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
		CreatedAt:    now,
		ExpiresAt:    now.Add(c.ttl),
	}

	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
}

// GetStats returns telemetry metrics
func (c *MemoryCache) GetStats() Stats {
	c.mu.RLock()
	count := len(c.entries)
	c.mu.RUnlock()

	h := atomic.LoadInt64(&c.hits)
	m := atomic.LoadInt64(&c.misses)
	saved := atomic.LoadInt64(&c.tokensSaved)

	pct := 0
	if total := h + m; total > 0 {
		pct = int((h * 100) / total)
	}

	return Stats{
		Hits:           h,
		Misses:         m,
		TokensSaved:    saved,
		EntriesCount:   count,
		EstimatedCache: pct,
	}
}
