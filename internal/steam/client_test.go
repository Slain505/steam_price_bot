package steam

import "testing"

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
		tags []invTag
		want string
	}{
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Common_Weapon"}},
			"⬜",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Uncommon_Weapon"}},
			"🟦",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Rare_Weapon"}},
			"🔵",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Mythical_Weapon"}},
			"🟣",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Legendary_Weapon"}},
			"🔴",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Ancient_Weapon"}},
			"🟡",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Contraband"}},
			"🔶",
		},
		// Suffix stripping: _Character, _Equipment
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Rare_Character"}},
			"🔵",
		},
		{
			[]invTag{{Category: "Rarity", InternalName: "Rarity_Ancient_Equipment"}},
			"🟡",
		},
		// Non-rarity tag — should be ignored
		{
			[]invTag{{Category: "Type", InternalName: "Rifle"}},
			"",
		},
		// Empty tags
		{[]invTag{}, ""},
	}
	for _, tt := range tests {
		got := parseRarity(tt.tags)
		if got != tt.want {
			t.Errorf("parseRarity(%v) = %q, want %q", tt.tags, got, tt.want)
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
