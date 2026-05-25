package tracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/slain505/steam-price-bot/internal/pricecache"
	"github.com/slain505/steam-price-bot/internal/steam"
	"github.com/slain505/steam-price-bot/internal/storage"
)

// AlertFunc is called when a price alert triggers.
type AlertFunc func(userID int64, message string)

// AdminNotifyFunc is called for bot-operational alerts (cookie expiry, etc.).
// Implementations should route to the admin chat only.
type AdminNotifyFunc func(message string)

// Tracker runs a background goroutine that refreshes prices for all tracked
// inventories every hour and fires price alerts.
type Tracker struct {
	store        *storage.Storage
	steam        *steam.Client
	prices       *pricecache.Cache // Skinport bulk cache — fast, no rate limits
	onAlert      AlertFunc
	onAdminNotif AdminNotifyFunc
	quit         chan struct{}

	// lastRun is written by the tracker goroutine and read by the bot goroutine,
	// so access must be protected by a mutex to avoid a data race.
	runMu   sync.Mutex
	lastRun time.Time

	// tickMu prevents two ticks from running simultaneously (e.g. regular
	// hourly tick vs an admin-triggered force refresh).
	tickMu sync.Mutex

	// cookieAlertMu guards lastCookieAlert. Dedupes the cookie-stale admin
	// notification per steamID so we don't spam the operator every tick.
	cookieAlertMu sync.Mutex
	lastCookieAlert map[string]time.Time
}

// LastRun returns the time when the tracker last finished a full tick.
// Returns zero value (time.Time{}) if the tracker hasn't completed a run yet.
// Calling t.lastRun.IsZero() is how you check "has it run at all?".
func (t *Tracker) LastRun() time.Time {
	t.runMu.Lock()
	defer t.runMu.Unlock()
	return t.lastRun
}

func New(s *storage.Storage, sc *steam.Client, pc *pricecache.Cache) *Tracker {
	return &Tracker{
		store:           s,
		steam:           sc,
		prices:          pc,
		quit:            make(chan struct{}),
		lastCookieAlert: make(map[string]time.Time),
	}
}

func (t *Tracker) SetAlertFunc(fn AlertFunc)             { t.onAlert = fn }
func (t *Tracker) SetAdminNotifyFunc(fn AdminNotifyFunc) { t.onAdminNotif = fn }

// cookieAlertCooldown ensures the operator is poked at most once per day
// per steamID — otherwise every hourly tick would fire another notification.
const cookieAlertCooldown = 24 * time.Hour

// notifyCookieIssue is the operational alert path for "your Steam cookie
// looks stale". It dedupes per-steamID and is a no-op if no admin handler
// has been wired up.
func (t *Tracker) notifyCookieIssue(steamID, detail string) {
	if t.onAdminNotif == nil {
		return
	}
	t.cookieAlertMu.Lock()
	last, ok := t.lastCookieAlert[steamID]
	if ok && time.Since(last) < cookieAlertCooldown {
		t.cookieAlertMu.Unlock()
		return
	}
	t.lastCookieAlert[steamID] = time.Now()
	t.cookieAlertMu.Unlock()

	msg := fmt.Sprintf(
		"⚠️ <b>Cookie Alert</b>\n\n"+
			"SteamID: <code>%s</code>\n"+
			"%s\n\n"+
			"<i>Refresh STEAM_SESSION_COOKIE in .env:\n"+
			"F12 → Application → Cookies → steamcommunity.com → copy steamLoginSecure → restart bot.</i>",
		steamID, detail,
	)
	t.onAdminNotif(msg)
}

// FireTestAlerts sends a test alert for every configured alert, ignoring the
// threshold. Useful for admins to verify the alert pipeline end-to-end.
func (t *Tracker) FireTestAlerts() {
	if t.onAlert == nil {
		return
	}
	alerts, err := t.store.GetAllAlerts()
	if err != nil {
		log.Printf("tracker: test alerts: %v", err)
		return
	}
	for _, a := range alerts {
		current, _, _ := t.store.GetLatestPrice(a.MarketHashName)
		var statusLine string
		if current == 0 {
			statusLine = "⚠️ No price in DB yet"
		} else {
			change := (current - a.BasePrice) / a.BasePrice * 100
			statusLine = fmt.Sprintf("%.2f → %.2f (%+.1f%%)", a.BasePrice, current, change)
		}
		mURL := "https://steamcommunity.com/market/listings/730/" + url.PathEscape(a.MarketHashName)
		msg := fmt.Sprintf(
			"🧪 <b>TEST ALERT</b>\n<a href=\"%s\">%s</a>\n%s\nThreshold: ±%.0f%%",
			mURL, a.MarketHashName, statusLine, a.Threshold,
		)
		t.onAlert(a.UserID, msg)
	}
}

// ForceRefresh triggers an immediate tick in a new goroutine.
// Returns false if a tick is already in progress.
func (t *Tracker) ForceRefresh() bool {
	if !t.tickMu.TryLock() {
		return false
	}
	t.tickMu.Unlock()
	go t.tick()
	return true
}

func (t *Tracker) Start() {
	go t.loop()
	go t.heartbeatLoop()
}

// heartbeatInterval is how often we deliver the "every alert and its current
// state" digest. Half-an-hour keeps the user reassured that the pipeline is
// alive without flooding the chat.
const heartbeatInterval = 30 * time.Minute

// heartbeatLoop sends a digest of every user's alerts on a fixed cadence.
// Unlike fireAlerts (threshold-gated), this fires even when nothing moved —
// useful for verifying the alert pipeline end-to-end.
func (t *Tracker) heartbeatLoop() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tracker: heartbeat panic: %v", r)
		}
	}()
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.sendHeartbeats()
		case <-t.quit:
			return
		}
	}
}

// sendHeartbeats groups alerts by user, then dispatches one consolidated
// digest message per user with the current price + delta + threshold state
// for every alert they have configured.
func (t *Tracker) sendHeartbeats() {
	if t.onAlert == nil {
		return
	}
	alerts, err := t.store.GetAllAlerts()
	if err != nil {
		log.Printf("tracker: heartbeat get alerts: %v", err)
		return
	}
	if len(alerts) == 0 {
		return
	}

	byUser := make(map[int64][]storage.Alert)
	for _, a := range alerts {
		byUser[a.UserID] = append(byUser[a.UserID], a)
	}

	for userID, userAlerts := range byUser {
		var sb strings.Builder
		sb.WriteString("📊 <b>30-min Alert Check</b>\n\n")
		for _, a := range userAlerts {
			current, _, _ := t.store.GetLatestPrice(a.MarketHashName)
			mURL := "https://steamcommunity.com/market/listings/730/" + url.PathEscape(a.MarketHashName)
			if current == 0 || a.BasePrice == 0 {
				fmt.Fprintf(&sb,
					"⚠️ <a href=\"%s\">%s</a>\n  No price data yet\n\n",
					mURL, a.MarketHashName,
				)
				continue
			}
			change := (current - a.BasePrice) / a.BasePrice * 100
			icon := "⏸"
			if change >= a.Threshold {
				icon = "📈"
			} else if change <= -a.Threshold {
				icon = "📉"
			} else if change > 0 {
				icon = "🔹"
			} else if change < 0 {
				icon = "🔸"
			}
			triggered := ""
			if math.Abs(change) >= a.Threshold {
				triggered = " <b>(triggered!)</b>"
			}
			fmt.Fprintf(&sb,
				"%s <a href=\"%s\">%s</a>%s\n  %.2f → %.2f (%+.1f%%)  threshold ±%.0f%%\n\n",
				icon, mURL, a.MarketHashName, triggered,
				a.BasePrice, current, change, a.Threshold,
			)
		}
		t.onAlert(userID, sb.String())
	}
}

func (t *Tracker) Stop() {
	close(t.quit)
}

func (t *Tracker) loop() {
	// Kick off an update right away, then repeat every hour.
	lastRun := t.store.GetTrackerLastRun()

	if lastRun.IsZero() || time.Since(lastRun) >= time.Hour {
		t.tick()
	} else {
		remaining := time.Hour - time.Since(lastRun)
		log.Printf("tracker: last run %s ago, next tick in %s",
			time.Since(lastRun).Truncate(time.Second),
			remaining.Truncate(time.Second),
		)
		select {
		case <-time.After(remaining):
			t.tick()
		case <-t.quit:
			return
		}
	}

	ticker := time.NewTicker(time.Hour)

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
	if !t.tickMu.TryLock() {
		log.Println("tracker: tick already in progress, skipping")
		return
	}
	defer t.tickMu.Unlock()

	log.Println("tracker: tick started")

	defer log.Println("tracker: tick done")

	defer func() {
		if r := recover(); r != nil {
			log.Printf("tracker: tick panic: %v", r)
		}
	}()

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

	// Record completion time — protected by mutex because LastRun() can
	// be called from a different goroutine (the bot handler goroutine).
	now := time.Now()

	t.runMu.Lock()
	t.lastRun = now
	t.runMu.Unlock()

	t.store.SaveTrackerLastRun(now)
}

func (t *Tracker) refreshInventory(steamID string) {
	items, err := t.steam.FetchInventory(steamID)
	if err != nil {
		log.Printf("tracker: fetch inventory %s: %v", steamID, err)
		// Steam returning success=0 for an inventory we previously read
		// successfully = either the user flipped to private OR our cookie
		// expired. Either way, the operator should know.
		if errors.Is(err, steam.ErrPrivate) {
			t.notifyCookieIssue(steamID,
				"Steam returned <code>success:0</code> — inventory privacy changed OR session cookie expired.")
		}
		return
	}

	// Stale-cookie heuristic: a real CS2 inventory rarely has &lt;20 unique items.
	// When Steam serves a cached/limited snapshot (e.g. expired JWT), we tend
	// to see a much smaller list. Warn but keep going — partial data is still
	// better than nothing.
	if len(items) > 0 && len(items) < 20 {
		t.notifyCookieIssue(steamID,
			fmt.Sprintf("Only <b>%d</b> items returned (expected more). Steam is likely serving a stale snapshot.", len(items)))
	}

	// Persist to inventory cache so bot commands can use it instantly.
	if data, jsonErr := json.Marshal(items); jsonErr == nil {
		_ = t.store.SaveInventoryCache(steamID, data)
	}

	// Currency name for Skinport lookup (e.g. "USD"). Falls back to "USD" if unknown.
	currency := t.steam.Currency.Name
	if currency == "" {
		currency = "USD"
	}

	skinportHits, steamHits, steamMiss := 0, 0, 0

	for _, item := range items {
		if !item.Marketable {
			continue
		}
		// Capture previous price before overwriting, for spike detection.
		prevPrice, _, _ := t.store.GetLatestPrice(item.MarketHashName)

		// ── Tier 1: Skinport bulk cache ──────────────────────────────────────
		// Fast, zero network calls, covers USD/EUR/GBP/PLN.
		// RUB/KZT/UAH are not supported by Skinport → falls through to Steam API.
		var price float64
		if t.prices != nil {
			price = t.prices.Get(item.MarketHashName, currency)
		}

		if price > 0 {
			skinportHits++
		} else {
			// ── Tier 2: Steam Market API (rate-limited, 1 req / 1.5 s) ───────
			var err error
			price, err = t.steam.FetchPriceWithRetry(item.MarketHashName, 0, 2)
			if err != nil {
				log.Printf("tracker: price %s: %v", item.MarketHashName, err)
				steamMiss++
				continue
			}
			steamHits++
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

	log.Printf("tracker: inventory %s — skinport:%d steam:%d miss:%d",
		steamID, skinportHits, steamHits, steamMiss)
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
