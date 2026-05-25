package pricecache

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

const (
	refreshEvery   = 6 * time.Hour
	requestTimeout = 60 * time.Second
)

// skinportURL is a var (not const) so tests can point it at httptest.NewServer.
var skinportURL = "https://api.skinport.com/v1/items?app_id=730&currency=%s"

// ErrRateLimit is returned by Refresh when Skinport responds with HTTP 429.
// Use errors.Is(err, ErrRateLimit) to test instead of substring matching.
var ErrRateLimit = errors.New("skinport rate limit")

// Cache holds bulk prices fetched from the Skinport API.
// It is safe for concurrent reads and periodic background writes.
type Cache struct {
	mu        sync.RWMutex
	data      map[string]map[string]float64 // currency → market_hash_name → price
	fetchedAt map[string]time.Time          // currency → last successful fetch time
	quit      chan struct{}
}

// skinportItem is the JSON shape returned by api.skinport.com/v1/items.
type skinportItem struct {
	MarketHashName string  `json:"market_hash_name"`
	Currency       string  `json:"currency"`
	SuggestedPrice float64 `json:"suggested_price"`
	MeanPrice      float64 `json:"mean_price"`
	MinPrice       float64 `json:"min_price"`
}

// New returns an initialised (but not yet populated) Cache.
func New() *Cache {
	return &Cache{
		data:      make(map[string]map[string]float64),
		fetchedAt: make(map[string]time.Time),
		quit:      make(chan struct{}),
	}
}

// Get returns the cached price for an item in the given currency (e.g. "USD").
// Returns 0 if the item or currency is not in the cache.
func (c *Cache) Get(marketHashName, currency string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if byCurrency, ok := c.data[strings.ToUpper(currency)]; ok {
		return byCurrency[marketHashName]
	}
	return 0
}

// FetchedAt returns when the cache for the given currency was last populated.
func (c *Cache) FetchedAt(currency string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.fetchedAt[strings.ToUpper(currency)]
	return t, ok
}

// skinportFetchDelay is the pause between consecutive Skinport requests when
// fetching multiple currencies to stay within the API rate limit.
const skinportFetchDelay = 10 * time.Second

// StartAutoRefresh loads prices for each currency synchronously, then schedules
// a periodic background refresh every refreshEvery interval.
// The initial fetch blocks startup so the cache is ready when the bot accepts
// its first user request.
// Pass nil (or empty slice) to skip — useful when the configured currency is
// not supported by Skinport.
func (c *Cache) StartAutoRefresh(currencies []string) {
	if len(currencies) == 0 {
		return
	}

	log.Printf("pricecache: loading initial prices for %v from Skinport...", currencies)
	for i, cur := range currencies {
		cur := strings.ToUpper(cur)
		if err := c.refreshWithRetry(cur); err != nil {
			log.Printf("pricecache: initial fetch %s failed: %v", cur, err)
		}
		if i < len(currencies)-1 {
			time.Sleep(skinportFetchDelay)
		}
	}
	log.Println("pricecache: initial load complete")

	go func() {
		ticker := time.NewTicker(refreshEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				for i, cur := range currencies {
					cur := strings.ToUpper(cur)
					if err := c.refreshWithRetry(cur); err != nil {
						log.Printf("pricecache: refresh %s: %v", cur, err)
					}
					if i < len(currencies)-1 {
						time.Sleep(skinportFetchDelay)
					}
				}
			case <-c.quit:
				return
			}
		}
	}()
}

// refreshWithRetry wraps Refresh with automatic back-off on ErrRateLimit.
// Retries up to maxRetries times with the given waits between attempts.
// Non-rate-limit errors are returned immediately.
func (c *Cache) refreshWithRetry(currency string) error {
	// Total worst-case wait: 10s + 20s = 30s. Down from 90s.
	waits := []time.Duration{10 * time.Second, 20 * time.Second}
	var lastErr error
	for attempt := 0; attempt <= len(waits); attempt++ {
		if attempt > 0 {
			wait := waits[attempt-1]
			log.Printf("pricecache: rate-limited on %s, retrying in %s (attempt %d/%d)",
				currency, wait, attempt, len(waits))
			time.Sleep(wait)
		}
		err := c.Refresh(currency)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrRateLimit) {
			return err
		}
		lastErr = err
	}
	return lastErr
}

// Stop shuts down the background refresh goroutine.
func (c *Cache) Stop() {
	close(c.quit)
}

// Refresh fetches all CS2 items for the given currency code from Skinport
// and stores them in the cache.
func (c *Cache) Refresh(currency string) error {
	currency = strings.ToUpper(currency)
	items, err := fetchSkinport(currency)
	if err != nil {
		return err
	}

	byName := make(map[string]float64, len(items))
	for _, it := range items {
		price := it.SuggestedPrice
		if price == 0 {
			price = it.MeanPrice
		}
		if price == 0 {
			price = it.MinPrice
		}
		if price > 0 {
			byName[it.MarketHashName] = price
		}
	}

	c.mu.Lock()
	c.data[currency] = byName
	c.fetchedAt[currency] = time.Now()
	c.mu.Unlock()

	log.Printf("pricecache: loaded %d items for %s", len(byName), currency)
	return nil
}

// fetchSkinport performs the HTTP request and decodes the Skinport response.
// Skinport requires Accept-Encoding: br (brotli) and returns brotli-compressed JSON.
func fetchSkinport(currency string) ([]skinportItem, error) {
	client := &http.Client{Timeout: requestTimeout}

	req, err := http.NewRequest("GET", fmt.Sprintf(skinportURL, currency), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept-Encoding", "br, gzip")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; steam-price-bot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", ErrRateLimit, string(body))
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("skinport HTTP %d: %s", resp.StatusCode, string(body))
	}

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	default:
		reader = resp.Body
	}

	var items []skinportItem
	if err := json.NewDecoder(reader).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return items, nil
}
