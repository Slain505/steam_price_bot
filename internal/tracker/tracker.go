package tracker

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"time"

	"github.com/slain505/steam-price-bot/internal/steam"
	"github.com/slain505/steam-price-bot/internal/storage"
)

// AlertFunc is called when a price alert triggers.
type AlertFunc func(userID int64, message string)

// Tracker runs a background goroutine that refreshes prices for all tracked
// inventories every hour and fires price alerts.
type Tracker struct {
	store   *storage.Storage
	steam   *steam.Client
	onAlert AlertFunc
	quit    chan struct{}
}

func New(s *storage.Storage, sc *steam.Client) *Tracker {
	return &Tracker{
		store: s,
		steam: sc,
		quit:  make(chan struct{}),
	}
}

func (t *Tracker) SetAlertFunc(fn AlertFunc) { t.onAlert = fn }

func (t *Tracker) Start() {
	go t.loop()
}

func (t *Tracker) Stop() {
	close(t.quit)
}

func (t *Tracker) loop() {
	// Kick off an update right away, then repeat every hour.
	t.tick()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.tick()
		case <-t.quit:
			return
		}
	}
}

func (t *Tracker) tick() {
	tracked, err := t.store.GetAllTrackedInventories()
	if err != nil {
		log.Printf("tracker: list inventories: %v", err)
		return
	}

	// Deduplicate steam IDs so we don't fetch the same inventory twice
	// (multiple users might track the same profile).
	seen := make(map[string]bool)
	for _, inv := range tracked {
		if seen[inv.SteamID] {
			continue
		}
		seen[inv.SteamID] = true
		t.refreshInventory(inv.SteamID)
	}

	if t.onAlert != nil {
		t.fireAlerts()
	}
}

func (t *Tracker) refreshInventory(steamID string) {
	items, err := t.steam.FetchInventory(steamID)
	if err != nil {
		log.Printf("tracker: fetch inventory %s: %v", steamID, err)
		return
	}

	// Persist to inventory cache so bot commands can use it instantly.
	if data, jsonErr := json.Marshal(items); jsonErr == nil {
		_ = t.store.SaveInventoryCache(steamID, data)
	}

	for _, item := range items {
		if !item.Marketable {
			continue
		}
		// Capture previous price before overwriting, for spike detection.
		prevPrice, _, _ := t.store.GetLatestPrice(item.MarketHashName)

		// Retry up to 2 times on rate limit (background task, longer sleep is fine).
		price, err := t.steam.FetchPriceWithRetry(item.MarketHashName, 0, 2)
		if err != nil {
			log.Printf("tracker: price %s: %v", item.MarketHashName, err)
			continue
		}
		if price > 0 {
			if err := t.store.SavePrice(item.MarketHashName, price); err != nil {
				log.Printf("tracker: save price: %v", err)
			}
			// Auto-alert on ≥ +5% spike.
			if t.onAlert != nil && prevPrice > 0 {
				change := (price - prevPrice) / prevPrice * 100
				if change >= 5.0 {
					t.fireSpikeAlert(steamID, item.MarketHashName, prevPrice, price, change)
				}
			}
		}
	}
}

// fireSpikeAlert notifies all users tracking steamID about a sudden price jump.
func (t *Tracker) fireSpikeAlert(steamID, name string, prev, cur, changePct float64) {
	tracked, err := t.store.GetAllTrackedInventories()
	if err != nil {
		return
	}
	mURL := "https://steamcommunity.com/market/listings/730/" + url.PathEscape(name)
	msg := fmt.Sprintf(
		"📈 <b>Price Spike +%.0f%%</b>\n<a href=\"%s\">%s</a>\n%.2f → %.2f",
		changePct, mURL, name, prev, cur,
	)
	for _, inv := range tracked {
		if inv.SteamID == steamID {
			t.onAlert(inv.UserID, msg)
		}
	}
}

func (t *Tracker) fireAlerts() {
	alerts, err := t.store.GetAllAlerts()
	if err != nil {
		log.Printf("tracker: get alerts: %v", err)
		return
	}

	for _, a := range alerts {
		current, _, err := t.store.GetLatestPrice(a.MarketHashName)
		if err != nil || current == 0 || a.BasePrice == 0 {
			continue
		}

		change := (current - a.BasePrice) / a.BasePrice * 100
		if math.Abs(change) < a.Threshold {
			continue
		}

		arrow := "📈"
		if change < 0 {
			arrow = "📉"
		}
		mURL := "https://steamcommunity.com/market/listings/730/" + url.PathEscape(a.MarketHashName)
		msg := fmt.Sprintf(
			"🔔 <b>Price Alert</b>\n%s <a href=\"%s\">%s</a>\nBase: %.2f → Now: %.2f (%+.1f%%)",
			arrow, mURL, a.MarketHashName, a.BasePrice, current, change,
		)
		t.onAlert(a.UserID, msg)
	}
}
