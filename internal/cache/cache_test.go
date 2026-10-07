package cache

import (
	"testing"
	"time"
)

func TestMemoryCache_Basic(t *testing.T) {
	c := NewMemoryCache(100 * time.Millisecond)

	key := ComputeCacheKey("gemini-3.8-flash", []string{"hi"}, nil)
	chunks := [][]byte{[]byte("data: chunk1\n\n"), []byte("data: [DONE]\n\n")}

	c.Set(key, chunks, []byte("full response"), 100, 20, 120)

	entry, found := c.Get(key)
	if !found {
		t.Fatalf("expected cache hit")
	}

	if len(entry.Chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(entry.Chunks))
	}

	stats := c.GetStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.TokensSaved != 100 {
		t.Errorf("expected 100 tokens saved, got %d", stats.TokensSaved)
	}

	time.Sleep(150 * time.Millisecond)
	_, foundAfterExpiry := c.Get(key)
	if foundAfterExpiry {
		t.Errorf("expected cache miss after expiration")
	}
}
