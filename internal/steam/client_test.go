package steam

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withInventoryServer overrides inventoryBaseURL with an httptest server.
// Returns a cleanup function.
func withInventoryServer(t *testing.T, h http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(h)
	original := inventoryBaseURL
	inventoryBaseURL = srv.URL
	return func() {
		inventoryBaseURL = original
		srv.Close()
	}
}

// --- parsePrice ---

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		// US format
		{"$14.50", 14.50},
		{"14.50", 14.50},
		{"1,234.56", 1234.56},
		// European format (comma as decimal separator)
		{"14,50 €", 14.50},
		{"1.234,56", 1234.56},
		// Ruble-style
		{"1 500,00 ₽", 1500.00},
		// Simple
		{"0.99", 0.99},
		{"100", 100.0},
		// Edge cases
		{"", 0},
		{"abc", 0},
		{"--", 0},
	}
	for _, tt := range tests {
		got := parsePrice(tt.input)
		if got != tt.want {
			t.Errorf("parsePrice(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- isValidSteamID ---

func TestIsValidSteamID(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"76561198000000000", true},
		{"76561199123456789", true},
		// Wrong prefix
		{"12345678901234567", false},
		// Too short
		{"7656119", false},
		// Too long
		{"765611980000000001", false},
		// Non-digits
		{"7656119abcdefghij", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isValidSteamID(tt.input)
		if got != tt.want {
			t.Errorf("isValidSteamID(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- parseRarity ---

func TestParseRarity(t *testing.T) {
	tests := []struct {
		name string
		tags []invTag
		want string
	}{
		{
			"AK-47 | Redline",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Common_Weapon"}},
			"⬜",
		},
		{
			"P250 | Sand Dune",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Uncommon_Weapon"}},
			"🩵",
		},
		{
			"MP9 | Storm",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Rare_Weapon"}},
			"🔵",
		},
		{
			"AWP | Pit Viper",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Mythical_Weapon"}},
			"🟣",
		},
		{
			// Covert weapon — Legendary maps to pink heart
			"M4A4 | Asiimov",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Legendary_Weapon"}},
			"🩷",
		},
		{
			// Ancient + knife (★ prefix) → yellow (knife/gloves)
			"★ Karambit | Fade",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Ancient_Weapon"}},
			"🟡",
		},
		{
			// Ancient + regular name → red (covert skin)
			"AWP | Dragon Lore",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Ancient_Weapon"}},
			"🔴",
		},
		{
			"M4A4 | Howl",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Contraband"}},
			"⭐",
		},
		// Suffix stripping: _Character, _Equipment
		{
			"Special Agent Ava",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Rare_Character"}},
			"🔵",
		},
		{
			"★ Sport Gloves",
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Ancient_Equipment"}},
			"🟡",
		},
		// Non-rarity tag — should be ignored
		{
			"AK-47 | Redline",
			[]invTag{{Category: "Type", InternalName: "Rifle"}},
			"",
		},
		// Empty tags
		{"Some Item", []invTag{}, ""},
	}
	for _, tt := range tests {
		got := parseRarity(tt.name, tt.tags)
		if got != tt.want {
			t.Errorf("parseRarity(%q, %v) = %q, want %q", tt.name, tt.tags, got, tt.want)
		}
	}
}

// --- stripHTML ---

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<b>hello</b>", "hello"},
		{"no tags here", "no tags here"},
		{"<a href=\"x\">link</a>", "link"},
		{"", ""},
		{"Sticker: <b>Team Liquid</b>", "Sticker: Team Liquid"},
		{"<br>", ""},
		{"a<b>b</b>c", "abc"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- parseDescEntries ---

func TestParseDescEntries_Stickers(t *testing.T) {
	entries := []invDescEntry{
		{Value: "Sticker: Fnatic, NaVi, Cloud9"},
	}
	stickers, charm, nameTag := parseDescEntries(entries)
	if len(stickers) != 3 {
		t.Errorf("want 3 stickers, got %d: %v", len(stickers), stickers)
	}
	if stickers[0] != "Fnatic" || stickers[1] != "NaVi" || stickers[2] != "Cloud9" {
		t.Errorf("unexpected sticker names: %v", stickers)
	}
	if charm != "" || nameTag != "" {
		t.Errorf("charm/nametag should be empty, got charm=%q tag=%q", charm, nameTag)
	}
}

func TestParseDescEntries_Charm(t *testing.T) {
	entries := []invDescEntry{
		{Value: "Charm: Hot Sauce"},
	}
	_, charm, _ := parseDescEntries(entries)
	if charm != "Hot Sauce" {
		t.Errorf("charm = %q, want %q", charm, "Hot Sauce")
	}
}

func TestParseDescEntries_NameTag(t *testing.T) {
	tests := []struct {
		value   string
		wantTag string
	}{
		{`Name Tag: ''Rush B''`, "Rush B"},
		{`Name Tag: "Clutch King"`, "Clutch King"},
		{`Name Tag: plain`, "plain"},
	}
	for _, tt := range tests {
		entries := []invDescEntry{{Value: tt.value}}
		_, _, nameTag := parseDescEntries(entries)
		if nameTag != tt.wantTag {
			t.Errorf("parseDescEntries(%q) nameTag = %q, want %q", tt.value, nameTag, tt.wantTag)
		}
	}
}

func TestParseDescEntries_Mixed(t *testing.T) {
	entries := []invDescEntry{
		{Value: "Sticker: Virtus.pro"},
		{Value: "Charm: Chicken"},
		{Value: "Name Tag: ''Pro''"},
		{Value: "Exterior: Field-Tested"}, // should be ignored
	}
	stickers, charm, nameTag := parseDescEntries(entries)
	if len(stickers) != 1 || stickers[0] != "Virtus.pro" {
		t.Errorf("stickers: want [Virtus.pro], got %v", stickers)
	}
	if charm != "Chicken" {
		t.Errorf("charm = %q, want Chicken", charm)
	}
	if nameTag != "Pro" {
		t.Errorf("nameTag = %q, want Pro", nameTag)
	}
}

// --- FetchInventory dedup / aggregation ---

// TestFetchInventory_MarketableAggregation_BugRegression locks the fix for the
// silent-drop bug where two copies of the same MarketHashName carrying different
// Marketable flags would be aggregated using the FIRST asset's flag — meaning
// if the cooled-down copy iterated first, the whole stack was filtered out.
func TestFetchInventory_MarketableAggregation_BugRegression(t *testing.T) {
	// Same MarketHashName ("AK-47 | Redline"), two different (classid:instanceid)
	// states: one cooled-down (marketable=0), one normal (marketable=1).
	// JSON ordering puts the cooled-down asset FIRST so it's processed first.
	const body = `{
		"success": 1,
		"more_items": 0,
		"assets": [
			{"classid":"100","instanceid":"cooldown","assetid":"a1","amount":"1"},
			{"classid":"100","instanceid":"normal","assetid":"a2","amount":"1"}
		],
		"descriptions": [
			{
				"classid":"100","instanceid":"cooldown",
				"market_hash_name":"AK-47 | Redline","name":"AK-47 | Redline",
				"type":"Rifle","tradable":0,"marketable":0,
				"descriptions":[],"tags":[]
			},
			{
				"classid":"100","instanceid":"normal",
				"market_hash_name":"AK-47 | Redline","name":"AK-47 | Redline",
				"type":"Rifle","tradable":1,"marketable":1,
				"descriptions":[],"tags":[]
			}
		]
	}`

	defer withInventoryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body))
	})()

	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	items, err := c.FetchInventory("76561198000000000")
	if err != nil {
		t.Fatalf("FetchInventory: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("want 1 deduped item, got %d", len(items))
	}
	got := items[0]
	if got.Amount != 2 {
		t.Errorf("Amount: want 2 (aggregated), got %d", got.Amount)
	}
	// The bug: aggregate inherited the FIRST asset's marketable=false.
	// The fix: OR across all copies → marketable=true.
	if !got.Marketable {
		t.Errorf("Marketable: want true (at least one copy is marketable), got false — BUG REGRESSION")
	}
	if !got.Tradable {
		t.Errorf("Tradable: want true (at least one copy is tradable), got false — BUG REGRESSION")
	}
}

func TestFetchInventory_AllCopiesNonMarketable(t *testing.T) {
	// Sanity check: if NO copy is marketable, the aggregate should remain non-marketable.
	const body = `{
		"success": 1,
		"more_items": 0,
		"assets": [
			{"classid":"200","instanceid":"x","assetid":"a1","amount":"1"},
			{"classid":"200","instanceid":"y","assetid":"a2","amount":"1"}
		],
		"descriptions": [
			{
				"classid":"200","instanceid":"x",
				"market_hash_name":"Operation Coin","name":"Operation Coin",
				"type":"Coin","tradable":0,"marketable":0,
				"descriptions":[],"tags":[]
			},
			{
				"classid":"200","instanceid":"y",
				"market_hash_name":"Operation Coin","name":"Operation Coin",
				"type":"Coin","tradable":0,"marketable":0,
				"descriptions":[],"tags":[]
			}
		]
	}`

	defer withInventoryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body))
	})()

	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	items, err := c.FetchInventory("76561198000000000")
	if err != nil {
		t.Fatalf("FetchInventory: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].Marketable {
		t.Error("aggregate of non-marketable copies should remain non-marketable")
	}
	if items[0].Amount != 2 {
		t.Errorf("Amount: want 2, got %d", items[0].Amount)
	}
}

func TestFetchInventory_PrivateInventory(t *testing.T) {
	defer withInventoryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"success":0}`))
	})()

	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	_, err := c.FetchInventory("76561198000000000")
	if err != ErrPrivate {
		t.Errorf("want ErrPrivate, got %v", err)
	}
}

func TestFetchInventory_InvalidSteamID(t *testing.T) {
	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	_, err := c.FetchInventory("not-a-steamid")
	if err != ErrInvalidSteamID {
		t.Errorf("want ErrInvalidSteamID, got %v", err)
	}
}

func TestFetchInventory_RateLimit(t *testing.T) {
	defer withInventoryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	})()

	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	_, err := c.FetchInventory("76561198000000000")
	if err != ErrRateLimit {
		t.Errorf("want ErrRateLimit, got %v", err)
	}
}

// Sanity: confirm a "%s" hint won't be appended by accident — fmt-style URL
// formatting is only in pricecache, not here.
func TestInventoryBaseURL_NoFormatVerbs(t *testing.T) {
	if got := fmt.Sprintf(inventoryBaseURL); got != inventoryBaseURL {
		t.Errorf("inventoryBaseURL contains format verbs: %q vs %q", got, inventoryBaseURL)
	}
}

// TestFetchInventory_AssetWithoutDescriptor verifies that assets whose
// classid:instanceid lacks a matching descriptor are silently skipped.
// This is currently expected behaviour but the test pins it so a future change
// (e.g. adding a default fallback) doesn't accidentally include unparseable items.
func TestFetchInventory_AssetWithoutDescriptor(t *testing.T) {
	// 3 assets: 2 have descriptors, 1 (instanceid="orphan") doesn't.
	const body = `{
		"success": 1,
		"more_items": 0,
		"assets": [
			{"classid":"100","instanceid":"normal","assetid":"a1","amount":"1"},
			{"classid":"100","instanceid":"orphan","assetid":"a2","amount":"1"},
			{"classid":"200","instanceid":"normal","assetid":"a3","amount":"1"}
		],
		"descriptions": [
			{
				"classid":"100","instanceid":"normal",
				"market_hash_name":"Item A","name":"Item A",
				"type":"Rifle","tradable":1,"marketable":1,
				"descriptions":[],"tags":[]
			},
			{
				"classid":"200","instanceid":"normal",
				"market_hash_name":"Item B","name":"Item B",
				"type":"Pistol","tradable":1,"marketable":1,
				"descriptions":[],"tags":[]
			}
		]
	}`

	defer withInventoryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body))
	})()

	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	items, err := c.FetchInventory("76561198000000000")
	if err != nil {
		t.Fatalf("FetchInventory: %v", err)
	}

	// 2 unique types from 2 valid assets; the orphan was skipped.
	if len(items) != 2 {
		t.Errorf("want 2 items (1 orphan skipped), got %d: %v", len(items), itemNames(items))
	}
}

// TestFetchInventory_PaginatedDescriptorSharing checks that a descriptor
// loaded on page 1 is still available for matching assets on page 2.
// This is the most common silent-loss scenario: paginated inventories where
// the same item type appears on multiple pages.
func TestFetchInventory_PaginatedDescriptorSharing(t *testing.T) {
	var pageCount int
	const page1 = `{
		"success": 1,
		"more_items": 1,
		"last_assetid": "a1",
		"assets": [
			{"classid":"100","instanceid":"x","assetid":"a1","amount":"1"}
		],
		"descriptions": [
			{
				"classid":"100","instanceid":"x",
				"market_hash_name":"AK-47 | Redline","name":"AK-47 | Redline",
				"type":"Rifle","tradable":1,"marketable":1,
				"descriptions":[],"tags":[]
			}
		]
	}`
	const page2 = `{
		"success": 1,
		"more_items": 0,
		"assets": [
			{"classid":"100","instanceid":"x","assetid":"a2","amount":"1"}
		],
		"descriptions": [
			{
				"classid":"100","instanceid":"x",
				"market_hash_name":"AK-47 | Redline","name":"AK-47 | Redline",
				"type":"Rifle","tradable":1,"marketable":1,
				"descriptions":[],"tags":[]
			}
		]
	}`

	defer withInventoryServer(t, func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		w.WriteHeader(200)
		if pageCount == 1 {
			_, _ = w.Write([]byte(page1))
		} else {
			_, _ = w.Write([]byte(page2))
		}
	})()

	c := NewClient("", Currency{Code: 1, Symbol: "$", Name: "USD"})
	items, err := c.FetchInventory("76561198000000000")
	if err != nil {
		t.Fatalf("FetchInventory: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("want 1 deduped item across 2 pages, got %d", len(items))
	}
	if items[0].Amount != 2 {
		t.Errorf("Amount: want 2 (1 per page), got %d", items[0].Amount)
	}
	if pageCount < 2 {
		t.Errorf("expected at least 2 page fetches, got %d", pageCount)
	}
}

// itemNames is a tiny helper for nicer test failure messages.
func itemNames(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.MarketHashName
	}
	return out
}
