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
