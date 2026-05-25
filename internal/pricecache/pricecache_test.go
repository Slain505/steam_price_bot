package pricecache

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// withServer spins up an httptest server that returns the given handler and
// re-points skinportURL at it for the duration of the test.
// Returns a cleanup func to restore the original URL.
func withServer(t *testing.T, h http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(h)
	original := skinportURL
	skinportURL = srv.URL + "?app_id=730&currency=%s"
	return func() {
		skinportURL = original
		srv.Close()
	}
}

// gzipBody encodes a Skinport JSON response as gzip (the simpler of the two
// content encodings we support; brotli would need a separate writer).
func gzipBody(t *testing.T, items []skinportItem) []byte {
	t.Helper()
	var buf strings.Builder
	gz := gzip.NewWriter(&strings.Builder{})
	_ = gz // shadow to satisfy linter — we need bytes
	bb := newGzipBytes(t, items)
	_ = buf
	return bb
}

func newGzipBytes(t *testing.T, items []skinportItem) []byte {
	t.Helper()
	var raw []byte
	if items == nil {
		raw = []byte("null")
	} else {
		var err error
		raw, err = json.Marshal(items)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
	}
	// Wrap in gzip.
	var out bytesBuffer
	gz := gzip.NewWriter(&out)
	if _, err := gz.Write(raw); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return out.b
}

// bytesBuffer is a minimal io.Writer wrapper around []byte to avoid pulling in bytes pkg.
type bytesBuffer struct{ b []byte }

func (w *bytesBuffer) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

// ---------- Get ----------

func TestGet_EmptyCache(t *testing.T) {
	c := New()
	if got := c.Get("AK-47 | Redline", "USD"); got != 0 {
		t.Errorf("empty cache should return 0, got %v", got)
	}
}

func TestGet_CaseInsensitiveCurrency(t *testing.T) {
	c := New()
	// Manually populate to bypass HTTP.
	c.data["USD"] = map[string]float64{"AK-47 | Redline": 42.5}

	if got := c.Get("AK-47 | Redline", "usd"); got != 42.5 {
		t.Errorf("lowercase currency: got %v, want 42.5", got)
	}
	if got := c.Get("AK-47 | Redline", "USD"); got != 42.5 {
		t.Errorf("uppercase currency: got %v, want 42.5", got)
	}
}

func TestGet_MissingItem(t *testing.T) {
	c := New()
	c.data["USD"] = map[string]float64{"AK-47 | Redline": 42.5}

	if got := c.Get("Nonexistent Item", "USD"); got != 0 {
		t.Errorf("missing item should return 0, got %v", got)
	}
}

// ---------- Refresh ----------

func TestRefresh_PopulatesCache(t *testing.T) {
	defer withServer(t, func(w http.ResponseWriter, r *http.Request) {
		body := newGzipBytes(t, []skinportItem{
			{MarketHashName: "AK-47 | Redline", SuggestedPrice: 25.50},
			{MarketHashName: "AWP | Asiimov", SuggestedPrice: 100.0},
		})
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(200)
		_, _ = w.Write(body)
	})()

	c := New()
	if err := c.Refresh("USD"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got := c.Get("AK-47 | Redline", "USD"); got != 25.50 {
		t.Errorf("AK-47 price: got %v, want 25.50", got)
	}
	if got := c.Get("AWP | Asiimov", "USD"); got != 100.0 {
		t.Errorf("AWP price: got %v, want 100.0", got)
	}

	// FetchedAt should be populated.
	at, ok := c.FetchedAt("USD")
	if !ok {
		t.Error("FetchedAt should report cache loaded")
	}
	if time.Since(at) > 5*time.Second {
		t.Errorf("FetchedAt timestamp too old: %v", at)
	}
}

func TestRefresh_PriceFallback(t *testing.T) {
	// Items without suggested_price should fall back to mean_price, then min_price.
	defer withServer(t, func(w http.ResponseWriter, r *http.Request) {
		body := newGzipBytes(t, []skinportItem{
			{MarketHashName: "Only-Mean", MeanPrice: 10.0},
			{MarketHashName: "Only-Min", MinPrice: 5.0},
			{MarketHashName: "No-Price"},
		})
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(200)
		_, _ = w.Write(body)
	})()

	c := New()
	if err := c.Refresh("USD"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got := c.Get("Only-Mean", "USD"); got != 10.0 {
		t.Errorf("Only-Mean: got %v, want 10.0", got)
	}
	if got := c.Get("Only-Min", "USD"); got != 5.0 {
		t.Errorf("Only-Min: got %v, want 5.0", got)
	}
	if got := c.Get("No-Price", "USD"); got != 0 {
		t.Errorf("No-Price should be skipped, got %v", got)
	}
}

func TestRefresh_RateLimitReturnsSentinel(t *testing.T) {
	defer withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"errors":[{"id":"rate_limit_exceeded"}]}`))
	})()

	c := New()
	err := c.Refresh("USD")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRateLimit) {
		t.Errorf("expected ErrRateLimit, got %v", err)
	}
}

func TestRefresh_HTTPErrorIsNotRateLimit(t *testing.T) {
	defer withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`server error`))
	})()

	c := New()
	err := c.Refresh("USD")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrRateLimit) {
		t.Errorf("500 should NOT match ErrRateLimit, got %v", err)
	}
}

// ---------- refreshWithRetry ----------

func TestRefreshWithRetry_SucceedsAfterRateLimit(t *testing.T) {
	var attempts int
	defer withServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// First attempt: 429.
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`rate_limit_exceeded`))
			return
		}
		// Second attempt: success.
		body := newGzipBytes(t, []skinportItem{
			{MarketHashName: "X", SuggestedPrice: 1.0},
		})
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(200)
		_, _ = w.Write(body)
	})()

	// Shorten the waits so the test doesn't block 10+ seconds.
	// We can't easily inject this without refactoring, so use a sub-test that
	// accepts the cost — or test the success-on-first-try case below.
	c := New()
	t.Log("note: this test waits 10s on the retry path")
	if err := c.refreshWithRetry("USD"); err != nil {
		t.Fatalf("refreshWithRetry: %v", err)
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
	if got := c.Get("X", "USD"); got != 1.0 {
		t.Errorf("price after retry: got %v, want 1.0", got)
	}
}

func TestRefreshWithRetry_NonRateLimitErrorNotRetried(t *testing.T) {
	var attempts int
	defer withServer(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(500)
		_, _ = w.Write([]byte("server error"))
	})()

	c := New()
	err := c.refreshWithRetry("USD")
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("non-429 should not retry; got %d attempts", attempts)
	}
}

// ---------- StartAutoRefresh ----------

func TestStartAutoRefresh_EmptyList_NoOp(t *testing.T) {
	c := New()
	// Must return immediately and not start any goroutine that needs cleanup.
	c.StartAutoRefresh(nil)
	c.StartAutoRefresh([]string{})
}

// ---------- Concurrency ----------

func TestCache_ConcurrentReadWrite(t *testing.T) {
	c := New()
	c.data["USD"] = map[string]float64{"Item": 1.0}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// 5 readers, 1 writer — runs for 100ms.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = c.Get("Item", "USD")
					_, _ = c.FetchedAt("USD")
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				c.mu.Lock()
				c.data["USD"]["Item"] = float64(i)
				c.fetchedAt["USD"] = time.Now()
				c.mu.Unlock()
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
	// No assertion — pass if -race doesn't report a data race.
}
