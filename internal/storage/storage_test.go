package storage

import (
	"testing"
	"time"
)

// newTestDB opens an in-memory SQLite DB and registers cleanup.
func newTestDB(t *testing.T) *Storage {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Currency ---

func TestCurrency(t *testing.T) {
	s := newTestDB(t)

	// Unknown user → default USD
	if got := s.GetCurrency(1); got != "USD" {
		t.Errorf("default currency = %q, want USD", got)
	}

	if err := s.SetCurrency(1, "EUR"); err != nil {
		t.Fatalf("SetCurrency: %v", err)
	}
	if got := s.GetCurrency(1); got != "EUR" {
		t.Errorf("GetCurrency = %q, want EUR", got)
	}

	// Update same user
	if err := s.SetCurrency(1, "RUB"); err != nil {
		t.Fatalf("SetCurrency update: %v", err)
	}
	if got := s.GetCurrency(1); got != "RUB" {
		t.Errorf("GetCurrency after update = %q, want RUB", got)
	}

	// Independent users
	if err := s.SetCurrency(2, "PLN"); err != nil {
		t.Fatalf("SetCurrency user2: %v", err)
	}
	if got := s.GetCurrency(1); got != "RUB" {
		t.Errorf("user1 currency changed unexpectedly: %q", got)
	}
}

// --- Language ---

func TestLang(t *testing.T) {
	s := newTestDB(t)

	if got := s.GetLang(99); got != "en" {
		t.Errorf("default lang = %q, want en", got)
	}

	if err := s.SetLang(1, "ru"); err != nil {
		t.Fatalf("SetLang: %v", err)
	}
	if got := s.GetLang(1); got != "ru" {
		t.Errorf("GetLang = %q, want ru", got)
	}

	if err := s.SetLang(1, "de"); err != nil {
		t.Fatalf("SetLang update: %v", err)
	}
	if got := s.GetLang(1); got != "de" {
		t.Errorf("GetLang after update = %q, want de", got)
	}
}

// --- Onboarding ---

func TestOnboarding(t *testing.T) {
	s := newTestDB(t)

	if s.IsOnboarded(1) {
		t.Error("new user should not be onboarded")
	}
	if err := s.SetOnboarded(1); err != nil {
		t.Fatalf("SetOnboarded: %v", err)
	}
	if !s.IsOnboarded(1) {
		t.Error("user should be onboarded after SetOnboarded")
	}
	// Idempotent
	if err := s.SetOnboarded(1); err != nil {
		t.Fatalf("SetOnboarded second call: %v", err)
	}
	if !s.IsOnboarded(1) {
		t.Error("should still be onboarded")
	}
}

// --- Accounts ---

func TestAccounts(t *testing.T) {
	s := newTestDB(t)
	const uid = int64(1)

	// Empty state
	accs, err := s.GetAccounts(uid)
	if err != nil || len(accs) != 0 {
		t.Fatalf("GetAccounts empty: want [], got %v err=%v", accs, err)
	}

	// Add two accounts
	if err := s.AddAccount(uid, "76561198000000001", "Main"); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if err := s.AddAccount(uid, "76561198000000002", "Alt"); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}

	accs, err = s.GetAccounts(uid)
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(accs) != 2 {
		t.Errorf("want 2 accounts, got %d", len(accs))
	}

	// HasAccount
	ok, err := s.HasAccount(uid, "76561198000000001")
	if err != nil || !ok {
		t.Errorf("HasAccount known: want true, got %v err=%v", ok, err)
	}
	ok, err = s.HasAccount(uid, "76561199999999999")
	if err != nil || ok {
		t.Errorf("HasAccount unknown: want false, got %v err=%v", ok, err)
	}

	// GetAccountName
	if name := s.GetAccountName(uid, "76561198000000001"); name != "Main" {
		t.Errorf("GetAccountName = %q, want Main", name)
	}
	// Unknown ID falls back to the steamID string
	if name := s.GetAccountName(uid, "76561199999999999"); name != "76561199999999999" {
		t.Errorf("GetAccountName fallback = %q, want steamID", name)
	}

	// RemoveAccount
	id := accs[0].ID
	if err := s.RemoveAccount(uid, id); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	accs, _ = s.GetAccounts(uid)
	if len(accs) != 1 {
		t.Errorf("after remove: want 1 account, got %d", len(accs))
	}

	// Duplicate SteamID should fail (UNIQUE constraint)
	if err := s.AddAccount(uid, accs[0].SteamID, "Dup"); err == nil {
		t.Error("expected error on duplicate SteamID, got nil")
	}
}

// --- Price history ---

func TestPriceHistory(t *testing.T) {
	s := newTestDB(t)

	// No data → zeros
	price, at, err := s.GetLatestPrice("no::such::item")
	if err != nil || price != 0 || !at.IsZero() {
		t.Errorf("GetLatestPrice empty: want (0, zero, nil), got (%v, %v, %v)", price, at, err)
	}

	// Save two prices; latest should win
	if err := s.SavePrice("AK-47 | Redline", 15.50); err != nil {
		t.Fatalf("SavePrice: %v", err)
	}
	time.Sleep(2 * time.Millisecond) // ensure different fetched_at
	if err := s.SavePrice("AK-47 | Redline", 16.00); err != nil {
		t.Fatalf("SavePrice: %v", err)
	}

	price, _, err = s.GetLatestPrice("AK-47 | Redline")
	if err != nil {
		t.Fatalf("GetLatestPrice: %v", err)
	}
	if price != 16.00 {
		t.Errorf("GetLatestPrice = %v, want 16.00", price)
	}
}

func TestGetPriceAt(t *testing.T) {
	s := newTestDB(t)

	// Price saved now
	if err := s.SavePrice("item", 10.00); err != nil {
		t.Fatalf("SavePrice: %v", err)
	}

	// Query 1s in the future — should find price
	p, err := s.GetPriceAt("item", time.Now().UTC().Add(time.Second))
	if err != nil || p != 10.00 {
		t.Errorf("GetPriceAt future: want 10.00, got %v err=%v", p, err)
	}

	// Query 10s in the past — should find nothing
	p, err = s.GetPriceAt("item", time.Now().UTC().Add(-10*time.Second))
	if err != nil || p != 0 {
		t.Errorf("GetPriceAt past: want 0, got %v err=%v", p, err)
	}

	// Unknown item
	p, err = s.GetPriceAt("nobody", time.Now().UTC().Add(time.Second))
	if err != nil || p != 0 {
		t.Errorf("GetPriceAt unknown: want 0, got %v err=%v", p, err)
	}
}

// --- Tracked inventories ---

func TestTrackedInventories(t *testing.T) {
	s := newTestDB(t)

	if err := s.TrackInventory(1, "76561198000000001"); err != nil {
		t.Fatalf("TrackInventory: %v", err)
	}
	if err := s.TrackInventory(2, "76561198000000001"); err != nil {
		t.Fatalf("TrackInventory user2: %v", err)
	}

	tracked, err := s.GetTrackedInventories(1)
	if err != nil || len(tracked) != 1 {
		t.Errorf("GetTrackedInventories user1: want 1, got %d err=%v", len(tracked), err)
	}

	all, err := s.GetAllTrackedInventories()
	if err != nil || len(all) != 2 {
		t.Errorf("GetAllTrackedInventories: want 2, got %d err=%v", len(all), err)
	}

	// Untrack
	if err := s.UntrackInventory(1, "76561198000000001"); err != nil {
		t.Fatalf("UntrackInventory: %v", err)
	}
	tracked, _ = s.GetTrackedInventories(1)
	if len(tracked) != 0 {
		t.Errorf("after untrack: want 0, got %d", len(tracked))
	}

	// Idempotent untrack (no error on missing row)
	if err := s.UntrackInventory(1, "76561198000000001"); err != nil {
		t.Errorf("untrack missing: unexpected error %v", err)
	}
}

// --- Alerts ---

func TestAlerts(t *testing.T) {
	s := newTestDB(t)

	alerts, err := s.GetAlerts(1)
	if err != nil || len(alerts) != 0 {
		t.Fatalf("GetAlerts empty: want [], got %v err=%v", alerts, err)
	}

	if err := s.AddAlert(1, "AK-47 | Redline", 10.0, 15.50); err != nil {
		t.Fatalf("AddAlert: %v", err)
	}
	if err := s.AddAlert(1, "Desert Eagle | Blaze", 5.0, 80.00); err != nil {
		t.Fatalf("AddAlert: %v", err)
	}

	alerts, err = s.GetAlerts(1)
	if err != nil || len(alerts) != 2 {
		t.Fatalf("GetAlerts: want 2, got %d err=%v", len(alerts), err)
	}
	if alerts[0].Threshold != 10.0 || alerts[0].BasePrice != 15.50 {
		t.Errorf("alert[0] fields mismatch: %+v", alerts[0])
	}

	// Remove
	if err := s.RemoveAlert(1, alerts[0].ID); err != nil {
		t.Fatalf("RemoveAlert: %v", err)
	}
	alerts, _ = s.GetAlerts(1)
	if len(alerts) != 1 {
		t.Errorf("after remove: want 1, got %d", len(alerts))
	}

	// GetAllAlerts spans users
	_ = s.AddAlert(2, "Knife", 20.0, 200.00)
	all, err := s.GetAllAlerts()
	if err != nil || len(all) != 2 {
		t.Errorf("GetAllAlerts: want 2, got %d err=%v", len(all), err)
	}

	// RemoveAlert wrong user — should be a no-op, not an error
	if err := s.RemoveAlert(2, alerts[0].ID); err != nil {
		t.Errorf("RemoveAlert wrong user: unexpected error %v", err)
	}
	alerts, _ = s.GetAlerts(1)
	if len(alerts) != 1 {
		t.Errorf("wrong-user remove should not delete user1 alert")
	}
}

// --- Entry prices ---

func TestEntryPriceIfNew(t *testing.T) {
	s := newTestDB(t)

	// First call sets the price
	if err := s.SetEntryPriceIfNew(1, "AK-47 | Redline", 15.00); err != nil {
		t.Fatalf("SetEntryPriceIfNew: %v", err)
	}
	// Second call must be ignored (INSERT OR IGNORE)
	if err := s.SetEntryPriceIfNew(1, "AK-47 | Redline", 99.00); err != nil {
		t.Fatalf("SetEntryPriceIfNew second: %v", err)
	}
	p, err := s.GetEntryPrice(1, "AK-47 | Redline")
	if err != nil || p != 15.00 {
		t.Errorf("entry price: want 15.00 (first wins), got %v err=%v", p, err)
	}
}

func TestUpdateEntryPrice(t *testing.T) {
	s := newTestDB(t)

	_ = s.SetEntryPriceIfNew(1, "AK-47 | Redline", 15.00)

	if err := s.UpdateEntryPrice(1, "AK-47 | Redline", 18.00); err != nil {
		t.Fatalf("UpdateEntryPrice: %v", err)
	}
	p, _ := s.GetEntryPrice(1, "AK-47 | Redline")
	if p != 18.00 {
		t.Errorf("UpdateEntryPrice: want 18.00, got %v", p)
	}

	// UpdateEntryPrice on a new item (no prior entry) should create a row
	if err := s.UpdateEntryPrice(1, "New Item", 5.00); err != nil {
		t.Fatalf("UpdateEntryPrice new item: %v", err)
	}
	p, _ = s.GetEntryPrice(1, "New Item")
	if p != 5.00 {
		t.Errorf("UpdateEntryPrice new: want 5.00, got %v", p)
	}
}

func TestGetEntryPricesMap(t *testing.T) {
	s := newTestDB(t)

	_ = s.SetEntryPriceIfNew(1, "AK-47 | Redline", 15.00)
	_ = s.SetEntryPriceIfNew(1, "Desert Eagle | Blaze", 85.00)

	m, err := s.GetEntryPricesMap(1, []string{
		"AK-47 | Redline",
		"Desert Eagle | Blaze",
		"Unknown Item",
	})
	if err != nil {
		t.Fatalf("GetEntryPricesMap: %v", err)
	}
	if m["AK-47 | Redline"] != 15.00 {
		t.Errorf("AK price = %v, want 15.00", m["AK-47 | Redline"])
	}
	if m["Desert Eagle | Blaze"] != 85.00 {
		t.Errorf("DE price = %v, want 85.00", m["Desert Eagle | Blaze"])
	}
	if _, ok := m["Unknown Item"]; ok {
		t.Error("map should not contain unknown item")
	}

	// Empty names slice — should return nil without error
	m2, err := s.GetEntryPricesMap(1, nil)
	if err != nil || m2 != nil {
		t.Errorf("GetEntryPricesMap nil: want (nil, nil), got (%v, %v)", m2, err)
	}
}

func TestGetAllEntryPrices(t *testing.T) {
	s := newTestDB(t)

	_ = s.SetEntryPriceIfNew(1, "Item A", 10.00)
	_ = s.SetEntryPriceIfNew(1, "Item B", 20.00)
	_ = s.SetEntryPriceIfNew(2, "Item A", 12.00) // different user, same item

	records, err := s.GetAllEntryPrices(1)
	if err != nil {
		t.Fatalf("GetAllEntryPrices: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("GetAllEntryPrices user1: want 2, got %d", len(records))
	}
	for _, r := range records {
		if r.UserID != 1 {
			t.Errorf("record has wrong userID %d", r.UserID)
		}
	}
}

// --- Report hour ---

func TestReportHour(t *testing.T) {
	s := newTestDB(t)

	// Default — disabled
	if h := s.GetReportHour(1); h != -1 {
		t.Errorf("default report hour = %d, want -1", h)
	}

	if err := s.SetReportHour(1, 8); err != nil {
		t.Fatalf("SetReportHour: %v", err)
	}
	if h := s.GetReportHour(1); h != 8 {
		t.Errorf("GetReportHour = %d, want 8", h)
	}

	// Update
	if err := s.SetReportHour(1, 20); err != nil {
		t.Fatalf("SetReportHour update: %v", err)
	}
	if h := s.GetReportHour(1); h != 20 {
		t.Errorf("GetReportHour after update = %d, want 20", h)
	}

	// Multiple users — GetReportUsers
	_ = s.SetReportHour(1, 8)
	_ = s.SetReportHour(2, 8)
	_ = s.SetReportHour(3, 12)

	users, err := s.GetReportUsers(8)
	if err != nil || len(users) != 2 {
		t.Errorf("GetReportUsers(8): want 2, got %d err=%v", len(users), err)
	}

	users, err = s.GetReportUsers(12)
	if err != nil || len(users) != 1 {
		t.Errorf("GetReportUsers(12): want 1, got %d err=%v", len(users), err)
	}

	users, err = s.GetReportUsers(0)
	if err != nil || len(users) != 0 {
		t.Errorf("GetReportUsers(0): want 0, got %d err=%v", len(users), err)
	}

	// Disable report
	_ = s.SetReportHour(1, -1)
	users, _ = s.GetReportUsers(8)
	if len(users) != 1 {
		t.Errorf("after disabling user1: GetReportUsers(8): want 1, got %d", len(users))
	}
}

// --- Tracker last run ---

func TestTrackerLastRun_EmptyReturnsZero(t *testing.T) {
	s := newTestDB(t)
	if got := s.GetTrackerLastRun(); !got.IsZero() {
		t.Errorf("empty DB: want zero time, got %v", got)
	}
}

func TestTrackerLastRun_Roundtrip(t *testing.T) {
	s := newTestDB(t)

	// Pick a value with fractional seconds — UNIX storage drops sub-second precision,
	// so we expect a truncated round-trip.
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveTrackerLastRun(now)

	got := s.GetTrackerLastRun()
	if !got.Equal(now) {
		t.Errorf("roundtrip mismatch: saved %v, got %v", now, got)
	}
}

func TestTrackerLastRun_Overwrite(t *testing.T) {
	s := newTestDB(t)

	t1 := time.Unix(1700000000, 0).UTC()
	t2 := time.Unix(1700000001, 0).UTC()

	s.SaveTrackerLastRun(t1)
	s.SaveTrackerLastRun(t2)

	got := s.GetTrackerLastRun()
	if !got.Equal(t2) {
		t.Errorf("overwrite failed: want %v, got %v", t2, got)
	}
}

// --- BotStats ---

func TestGetBotStats(t *testing.T) {
	s := newTestDB(t)

	// Empty DB
	stats := s.GetBotStats()
	if stats.UniquePrices != 0 || stats.TrackedAccounts != 0 || stats.CachedInventories != 0 {
		t.Errorf("empty stats: want all 0, got %+v", stats)
	}

	// Populate
	_ = s.SavePrice("Item A", 1.0)
	_ = s.SavePrice("Item A", 2.0) // duplicate — should count as 1 UniquePrices
	_ = s.SavePrice("Item B", 3.0)
	_ = s.TrackInventory(1, "76561198000000001")
	_ = s.TrackInventory(2, "76561198000000002")
	_ = s.SaveInventoryCache("76561198000000001", []byte(`[]`))

	stats = s.GetBotStats()
	if stats.UniquePrices != 2 {
		t.Errorf("UniquePrices: want 2, got %d", stats.UniquePrices)
	}
	if stats.TrackedAccounts != 2 {
		t.Errorf("TrackedAccounts: want 2, got %d", stats.TrackedAccounts)
	}
	if stats.CachedInventories != 1 {
		t.Errorf("CachedInventories: want 1, got %d", stats.CachedInventories)
	}
}

// --- Account ↔ Tracking sync ---

func TestRemoveAccount_AlsoUntracks(t *testing.T) {
	s := newTestDB(t)
	const uid = int64(1)

	// Add an account and start tracking it.
	if err := s.AddAccount(uid, "76561198000000001", "Main"); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if err := s.TrackInventory(uid, "76561198000000001"); err != nil {
		t.Fatalf("TrackInventory: %v", err)
	}

	accs, _ := s.GetAccounts(uid)
	if len(accs) != 1 {
		t.Fatalf("setup: expected 1 account, got %d", len(accs))
	}
	tracked, _ := s.GetTrackedInventories(uid)
	if len(tracked) != 1 {
		t.Fatalf("setup: expected 1 tracked, got %d", len(tracked))
	}

	// Removing the account should also remove the tracking row.
	if err := s.RemoveAccount(uid, accs[0].ID); err != nil {
		t.Fatalf("RemoveAccount: %v", err)
	}
	accs, _ = s.GetAccounts(uid)
	if len(accs) != 0 {
		t.Errorf("after RemoveAccount: want 0 accounts, got %d", len(accs))
	}
	tracked, _ = s.GetTrackedInventories(uid)
	if len(tracked) != 0 {
		t.Errorf("after RemoveAccount: want 0 tracked, got %d", len(tracked))
	}
}

func TestBackfillTrackingFromAccounts_EnrollsAll(t *testing.T) {
	s := newTestDB(t)

	// Two users, each with two accounts — none of them tracked.
	_ = s.AddAccount(1, "76561198000000001", "U1A1")
	_ = s.AddAccount(1, "76561198000000002", "U1A2")
	_ = s.AddAccount(2, "76561198000000003", "U2A1")
	_ = s.AddAccount(2, "76561198000000004", "U2A2")

	n, err := s.BackfillTrackingFromAccounts()
	if err != nil {
		t.Fatalf("BackfillTrackingFromAccounts: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 backfilled rows, got %d", n)
	}

	all, _ := s.GetAllTrackedInventories()
	if len(all) != 4 {
		t.Errorf("expected 4 tracked inventories, got %d", len(all))
	}
}

func TestBackfillTrackingFromAccounts_Idempotent(t *testing.T) {
	s := newTestDB(t)

	_ = s.AddAccount(1, "76561198000000001", "Main")
	// First call enrolls.
	n1, _ := s.BackfillTrackingFromAccounts()
	if n1 != 1 {
		t.Errorf("first call: want 1 row, got %d", n1)
	}
	// Second call must be a no-op.
	n2, _ := s.BackfillTrackingFromAccounts()
	if n2 != 0 {
		t.Errorf("second call: want 0 new rows (idempotent), got %d", n2)
	}
}

func TestBackfillTrackingFromAccounts_PreservesExistingTracking(t *testing.T) {
	s := newTestDB(t)

	// Account A is already explicitly tracked.
	_ = s.AddAccount(1, "76561198000000001", "Tracked")
	_ = s.TrackInventory(1, "76561198000000001")
	// Account B is added but never tracked.
	_ = s.AddAccount(1, "76561198000000002", "Untracked")

	// Backfill should add B without disrupting A.
	n, _ := s.BackfillTrackingFromAccounts()
	if n != 1 {
		t.Errorf("backfill added: want 1 (only B), got %d", n)
	}
	tracked, _ := s.GetTrackedInventories(1)
	if len(tracked) != 2 {
		t.Errorf("after backfill: want 2 tracked, got %d", len(tracked))
	}
}

// --- Inventory cache ---

func TestInventoryCache_RoundTrip(t *testing.T) {
	s := newTestDB(t)

	// No cache yet
	data, at, err := s.GetInventoryCache("76561198000000001")
	if err != nil || data != nil || !at.IsZero() {
		t.Errorf("empty cache: want (nil, zero, nil), got (%v, %v, %v)", data, at, err)
	}

	// Save and read back
	payload := []byte(`[{"market_hash_name":"AK-47 | Redline"}]`)
	if err := s.SaveInventoryCache("76561198000000001", payload); err != nil {
		t.Fatalf("SaveInventoryCache: %v", err)
	}

	data, at, err = s.GetInventoryCache("76561198000000001")
	if err != nil {
		t.Fatalf("GetInventoryCache: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("payload mismatch: want %s, got %s", payload, data)
	}
	if at.IsZero() {
		t.Errorf("cached_at should be set")
	}

	// Overwrite
	newPayload := []byte(`[{"market_hash_name":"AWP | Asiimov"}]`)
	if err := s.SaveInventoryCache("76561198000000001", newPayload); err != nil {
		t.Fatalf("SaveInventoryCache overwrite: %v", err)
	}
	data, _, _ = s.GetInventoryCache("76561198000000001")
	if string(data) != string(newPayload) {
		t.Errorf("overwrite: want %s, got %s", newPayload, data)
	}
}
