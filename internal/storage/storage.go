package storage

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Storage wraps SQLite and provides typed CRUD for prices, tracking, alerts, accounts, and settings.
type Storage struct {
	db *sql.DB
}

type TrackedInventory struct {
	UserID  int64
	SteamID string
	AddedAt time.Time
}

type Alert struct {
	ID             int64
	UserID         int64
	MarketHashName string
	Threshold      float64
	BasePrice      float64
}

// EntryPriceRecord stores the reference ("entry") price used for P&L calculations.
type EntryPriceRecord struct {
	UserID          int64
	MarketHashName  string
	EntryPrice      float64
	FirstSeenAt     time.Time
}

// Account represents a named Steam account stored for a Telegram user.
type Account struct {
	ID      int64
	UserID  int64
	SteamID string
	Name    string
	AddedAt time.Time
}

func New(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Storage{db: db}
	return s, s.migrate()
}

func (s *Storage) Close() error { return s.db.Close() }

func (s *Storage) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS user_settings (
			user_id       INTEGER PRIMARY KEY,
			currency_code TEXT    NOT NULL DEFAULT 'USD',
			lang          TEXT    NOT NULL DEFAULT 'en',
			onboarded     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS accounts (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id  INTEGER NOT NULL,
			steam_id TEXT    NOT NULL,
			name     TEXT    NOT NULL DEFAULT '',
			added_at DATETIME NOT NULL,
			UNIQUE(user_id, steam_id)
		)`,
		`CREATE TABLE IF NOT EXISTS price_history (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			market_hash_name TEXT    NOT NULL,
			price            REAL    NOT NULL,
			fetched_at       DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ph_item_time ON price_history(market_hash_name, fetched_at)`,
		`CREATE TABLE IF NOT EXISTS tracked_inventories (
			user_id  INTEGER NOT NULL,
			steam_id TEXT    NOT NULL,
			added_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, steam_id)
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          INTEGER NOT NULL,
			market_hash_name TEXT    NOT NULL,
			threshold        REAL    NOT NULL,
			base_price       REAL    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS entry_prices (
			user_id          INTEGER NOT NULL,
			market_hash_name TEXT    NOT NULL,
			entry_price      REAL    NOT NULL,
			first_seen_at    DATETIME NOT NULL,
			PRIMARY KEY (user_id, market_hash_name)
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_cache (
			steam_id  TEXT     PRIMARY KEY,
			items     TEXT     NOT NULL,
			cached_at DATETIME NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	// Add new columns to existing installations (ignored if already present).
	for _, m := range []string{
		`ALTER TABLE user_settings ADD COLUMN lang TEXT NOT NULL DEFAULT 'en'`,
		`ALTER TABLE user_settings ADD COLUMN onboarded INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE user_settings ADD COLUMN report_hour INTEGER NOT NULL DEFAULT -1`,
	} {
		s.db.Exec(m)
	}
	return nil
}

// --- Price history ---

func (s *Storage) SavePrice(name string, price float64) error {
	_, err := s.db.Exec(
		`INSERT INTO price_history (market_hash_name, price, fetched_at) VALUES (?, ?, ?)`,
		name, price, time.Now().UTC(),
	)
	return err
}

func (s *Storage) GetLatestPrice(name string) (float64, time.Time, error) {
	var price float64
	var at time.Time
	err := s.db.QueryRow(
		`SELECT price, fetched_at FROM price_history
		 WHERE market_hash_name = ? ORDER BY fetched_at DESC LIMIT 1`,
		name,
	).Scan(&price, &at)
	if err == sql.ErrNoRows {
		return 0, time.Time{}, nil
	}
	return price, at, err
}

func (s *Storage) GetPriceAt(name string, at time.Time) (float64, error) {
	var price float64
	err := s.db.QueryRow(
		`SELECT price FROM price_history
		 WHERE market_hash_name = ? AND fetched_at <= ?
		 ORDER BY fetched_at DESC LIMIT 1`,
		name, at,
	).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return price, err
}

// --- Tracking ---

func (s *Storage) TrackInventory(userID int64, steamID string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO tracked_inventories (user_id, steam_id, added_at) VALUES (?, ?, ?)`,
		userID, steamID, time.Now().UTC(),
	)
	return err
}

func (s *Storage) UntrackInventory(userID int64, steamID string) error {
	_, err := s.db.Exec(
		`DELETE FROM tracked_inventories WHERE user_id = ? AND steam_id = ?`,
		userID, steamID,
	)
	return err
}

func (s *Storage) GetTrackedInventories(userID int64) ([]TrackedInventory, error) {
	return s.scanTracked(`SELECT user_id, steam_id, added_at FROM tracked_inventories WHERE user_id = ?`, userID)
}

func (s *Storage) GetAllTrackedInventories() ([]TrackedInventory, error) {
	return s.scanTracked(`SELECT user_id, steam_id, added_at FROM tracked_inventories`)
}

func (s *Storage) scanTracked(query string, args ...any) ([]TrackedInventory, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackedInventory
	for rows.Next() {
		var t TrackedInventory
		if err := rows.Scan(&t.UserID, &t.SteamID, &t.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// --- User settings: currency ---

func (s *Storage) GetCurrency(userID int64) string {
	var code string
	err := s.db.QueryRow(
		`SELECT currency_code FROM user_settings WHERE user_id = ?`, userID,
	).Scan(&code)
	if err != nil {
		return "USD"
	}
	return code
}

func (s *Storage) SetCurrency(userID int64, code string) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, currency_code) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET currency_code = excluded.currency_code`,
		userID, code,
	)
	return err
}

// --- User settings: language ---

func (s *Storage) GetLang(userID int64) string {
	var lang string
	err := s.db.QueryRow(
		`SELECT lang FROM user_settings WHERE user_id = ?`, userID,
	).Scan(&lang)
	if err != nil {
		return "en"
	}
	return lang
}

func (s *Storage) SetLang(userID int64, lang string) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, lang) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET lang = excluded.lang`,
		userID, lang,
	)
	return err
}

// --- User settings: onboarding ---

func (s *Storage) IsOnboarded(userID int64) bool {
	var v int
	err := s.db.QueryRow(
		`SELECT onboarded FROM user_settings WHERE user_id = ?`, userID,
	).Scan(&v)
	return err == nil && v == 1
}

func (s *Storage) SetOnboarded(userID int64) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, onboarded) VALUES (?, 1)
		 ON CONFLICT(user_id) DO UPDATE SET onboarded = 1`,
		userID,
	)
	return err
}

// --- Accounts ---

func (s *Storage) GetAccounts(userID int64) ([]Account, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, steam_id, name, added_at FROM accounts WHERE user_id = ? ORDER BY added_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.UserID, &a.SteamID, &a.Name, &a.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Storage) AddAccount(userID int64, steamID, name string) error {
	_, err := s.db.Exec(
		`INSERT INTO accounts (user_id, steam_id, name, added_at) VALUES (?, ?, ?, ?)`,
		userID, steamID, name, time.Now().UTC(),
	)
	return err
}

func (s *Storage) RemoveAccount(userID, accountID int64) error {
	_, err := s.db.Exec(
		`DELETE FROM accounts WHERE id = ? AND user_id = ?`, accountID, userID,
	)
	return err
}

func (s *Storage) HasAccount(userID int64, steamID string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM accounts WHERE user_id = ? AND steam_id = ?`, userID, steamID,
	).Scan(&count)
	return count > 0, err
}

// GetAccountName returns the stored name for a steamID, or the steamID itself if not found.
func (s *Storage) GetAccountName(userID int64, steamID string) string {
	var name string
	err := s.db.QueryRow(
		`SELECT name FROM accounts WHERE user_id = ? AND steam_id = ?`, userID, steamID,
	).Scan(&name)
	if err != nil || name == "" {
		return steamID
	}
	return name
}

// --- Alerts ---

func (s *Storage) AddAlert(userID int64, name string, threshold, basePrice float64) error {
	_, err := s.db.Exec(
		`INSERT INTO alerts (user_id, market_hash_name, threshold, base_price) VALUES (?, ?, ?, ?)`,
		userID, name, threshold, basePrice,
	)
	return err
}

func (s *Storage) RemoveAlert(userID, alertID int64) error {
	_, err := s.db.Exec(`DELETE FROM alerts WHERE id = ? AND user_id = ?`, alertID, userID)
	return err
}

func (s *Storage) GetAlerts(userID int64) ([]Alert, error) {
	return s.scanAlerts(`SELECT id, user_id, market_hash_name, threshold, base_price FROM alerts WHERE user_id = ?`, userID)
}

func (s *Storage) GetAllAlerts() ([]Alert, error) {
	return s.scanAlerts(`SELECT id, user_id, market_hash_name, threshold, base_price FROM alerts`)
}

// --- Entry prices (P&L) ---

// SetEntryPriceIfNew records the entry price only on first encounter.
func (s *Storage) SetEntryPriceIfNew(userID int64, name string, price float64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO entry_prices (user_id, market_hash_name, entry_price, first_seen_at)
		 VALUES (?, ?, ?, ?)`,
		userID, name, price, time.Now().UTC(),
	)
	return err
}

// UpdateEntryPrice forcefully overwrites the entry price.
func (s *Storage) UpdateEntryPrice(userID int64, name string, price float64) error {
	_, err := s.db.Exec(
		`INSERT INTO entry_prices (user_id, market_hash_name, entry_price, first_seen_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(user_id, market_hash_name) DO UPDATE SET entry_price = excluded.entry_price`,
		userID, name, price, time.Now().UTC(),
	)
	return err
}

// GetEntryPrice returns the entry price for one item, or 0 if not set.
func (s *Storage) GetEntryPrice(userID int64, name string) (float64, error) {
	var p float64
	err := s.db.QueryRow(
		`SELECT entry_price FROM entry_prices WHERE user_id = ? AND market_hash_name = ?`,
		userID, name,
	).Scan(&p)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return p, err
}

// GetEntryPricesMap returns entry prices for a set of item names as a map.
func (s *Storage) GetEntryPricesMap(userID int64, names []string) (map[string]float64, error) {
	if len(names) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(names))
	args := make([]any, 0, len(names)+1)
	args = append(args, userID)
	for i, n := range names {
		placeholders[i] = "?"
		args = append(args, n)
	}
	q := `SELECT market_hash_name, entry_price FROM entry_prices
	      WHERE user_id = ? AND market_hash_name IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64, len(names))
	for rows.Next() {
		var name string
		var price float64
		if err := rows.Scan(&name, &price); err != nil {
			return nil, err
		}
		out[name] = price
	}
	return out, rows.Err()
}

// GetAllEntryPrices returns all entry price records for a user.
func (s *Storage) GetAllEntryPrices(userID int64) ([]EntryPriceRecord, error) {
	rows, err := s.db.Query(
		`SELECT user_id, market_hash_name, entry_price, first_seen_at
		 FROM entry_prices WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EntryPriceRecord
	for rows.Next() {
		var r EntryPriceRecord
		if err := rows.Scan(&r.UserID, &r.MarketHashName, &r.EntryPrice, &r.FirstSeenAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Storage) scanAlerts(query string, args ...any) ([]Alert, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.UserID, &a.MarketHashName, &a.Threshold, &a.BasePrice); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- Daily report schedule ---

// SetReportHour stores the UTC hour (0–23) for the daily report, or -1 to disable.
func (s *Storage) SetReportHour(userID int64, hour int) error {
	_, err := s.db.Exec(
		`INSERT INTO user_settings (user_id, report_hour) VALUES (?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET report_hour = excluded.report_hour`,
		userID, hour,
	)
	return err
}

// GetReportHour returns the configured report hour, or -1 if not set.
func (s *Storage) GetReportHour(userID int64) int {
	var h int
	err := s.db.QueryRow(
		`SELECT report_hour FROM user_settings WHERE user_id = ?`, userID,
	).Scan(&h)
	if err != nil {
		return -1
	}
	return h
}

// --- Inventory cache ---

// SaveInventoryCache stores a JSON-serialised inventory for a steamID.
func (s *Storage) SaveInventoryCache(steamID string, data []byte) error {
	_, err := s.db.Exec(
		`INSERT INTO inventory_cache (steam_id, items, cached_at) VALUES (?, ?, ?)
		 ON CONFLICT(steam_id) DO UPDATE SET items = excluded.items, cached_at = excluded.cached_at`,
		steamID, string(data), time.Now().UTC(),
	)
	return err
}

// GetInventoryCache returns the cached inventory bytes and the time it was saved.
// Returns (nil, zero, nil) if no cache entry exists.
func (s *Storage) GetInventoryCache(steamID string) ([]byte, time.Time, error) {
	var raw string
	var at time.Time
	err := s.db.QueryRow(
		`SELECT items, cached_at FROM inventory_cache WHERE steam_id = ?`, steamID,
	).Scan(&raw, &at)
	if err == sql.ErrNoRows {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	return []byte(raw), at, nil
}

// GetReportUsers returns all user IDs that have a daily report scheduled at the given UTC hour.
func (s *Storage) GetReportUsers(hour int) ([]int64, error) {
	rows, err := s.db.Query(
		`SELECT user_id FROM user_settings WHERE report_hour = ?`, hour,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
