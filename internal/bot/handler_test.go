package bot

import (
	"strings"
	"testing"
	"time"
)

// --- shortName ---

func TestShortName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"AK-47 | Redline (Field-Tested)", "AK-47 | Redline"},
		{"AK-47 | Redline (Factory New)", "AK-47 | Redline"},
		{"AK-47 | Redline (Minimal Wear)", "AK-47 | Redline"},
		{"AK-47 | Redline (Well-Worn)", "AK-47 | Redline"},
		{"AK-47 | Redline (Battle-Scarred)", "AK-47 | Redline"},
		{"★ Karambit | Fade (Factory New)", "Karambit | Fade"},
		{"★ StatTrak™ Karambit", "StatTrak™ Karambit"},
		{"Desert Eagle | Blaze", "Desert Eagle | Blaze"}, // no suffix
		{"Sticker | Fnatic", "Sticker | Fnatic"},
	}
	for _, tt := range tests {
		got := shortName(tt.input)
		if got != tt.want {
			t.Errorf("shortName(%q)\n  got  %q\n  want %q", tt.input, got, tt.want)
		}
	}
}

// --- esc ---

func TestEsc(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"AT&T", "AT&amp;T"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"<>&", "&lt;&gt;&amp;"},
		{"no special chars", "no special chars"},
		{"", ""},
	}
	for _, tt := range tests {
		got := esc(tt.input)
		if got != tt.want {
			t.Errorf("esc(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- itemBadge ---

func TestItemBadge(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		locked   bool
		contains []string
		absent   []string
	}{
		{
			name: "AK-47 | Redline (Field-Tested)", itemType: "Rifle",
			contains: []string{"FT"}, absent: []string{"⭐", "ST", "🔒"},
		},
		{
			name: "★ Karambit | Fade (Factory New)", itemType: "Knife",
			contains: []string{"⭐", "FN"}, absent: []string{"ST"},
		},
		{
			name: "StatTrak™ AK-47 | Redline (Minimal Wear)", itemType: "Rifle",
			contains: []string{"ST", "MW"}, absent: []string{"⭐"},
		},
		{
			name: "★ StatTrak™ Karambit | Doppler (Factory New)", itemType: "Knife",
			contains: []string{"⭐", "ST", "FN"},
		},
		{
			name: "AK-47 | Redline (Field-Tested)", itemType: "Rifle", locked: true,
			contains: []string{"FT", "🔒"},
		},
		{
			name: "Sticker | Fnatic", itemType: "Sticker",
			contains: []string{"🏷"}, absent: []string{"FT", "FN"},
		},
		{
			name: "Danger Zone Case", itemType: "Base Grade Container",
			contains: []string{"📦"},
		},
		{
			name: "Spray | Counter Terrorist", itemType: "Base Grade Graffiti",
			contains: []string{"🎨"},
		},
		{
			// No quality, no special type → empty badge
			name: "AK-47 | Redline", itemType: "Rifle",
			absent: []string{"FT", "FN", "MW", "WW", "BS", "⭐", "ST", "🔒"},
		},
	}
	for _, tt := range tests {
		got := itemBadge(tt.name, tt.itemType, tt.locked)
		for _, want := range tt.contains {
			if !strings.Contains(got, want) {
				t.Errorf("itemBadge(%q, %q, %v) = %q, should contain %q",
					tt.name, tt.itemType, tt.locked, got, want)
			}
		}
		for _, absent := range tt.absent {
			if strings.Contains(got, absent) {
				t.Errorf("itemBadge(%q, %q, %v) = %q, should NOT contain %q",
					tt.name, tt.itemType, tt.locked, got, absent)
			}
		}
	}
}

// --- parsePeriod ---

func TestParsePeriod(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		ok    bool
	}{
		{"24h", 24 * time.Hour, true},
		{"7d", 7 * 24 * time.Hour, true},
		{"30d", 30 * 24 * time.Hour, true},
		{"1h", 0, false},
		{"1d", 0, false},
		{"", 0, false},
		{"7D", 0, false},  // case-sensitive
		{"24H", 0, false}, // case-sensitive
	}
	for _, tt := range tests {
		got, ok := parsePeriod(tt.input)
		if ok != tt.ok {
			t.Errorf("parsePeriod(%q): ok = %v, want %v", tt.input, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("parsePeriod(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- isValidSteamID64 ---

func TestIsValidSteamID64(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"76561198000000000", true},
		{"76561199123456789", true},
		{"76561197960287930", true},  // well-known ID
		{"7656119", false},           // too short
		{"765611980000000001", false}, // too long
		{"12345678901234567", false},  // wrong prefix
		{"7656119abcdefghij", false},  // non-digits
		{"", false},
	}
	for _, tt := range tests {
		got := isValidSteamID64(tt.input)
		if got != tt.want {
			t.Errorf("isValidSteamID64(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- marketLink ---

func TestMarketLink(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			"AK-47 | Redline (Field-Tested)",
			"https://steamcommunity.com/market/listings/730/AK-47%20%7C%20Redline%20%28Field-Tested%29",
		},
		{
			"Desert Eagle | Blaze",
			"https://steamcommunity.com/market/listings/730/Desert%20Eagle%20%7C%20Blaze",
		},
		{
			"★ Karambit | Fade (Factory New)",
			"https://steamcommunity.com/market/listings/730/%E2%98%85%20Karambit%20%7C%20Fade%20%28Factory%20New%29",
		},
	}
	for _, tt := range tests {
		got := marketLink(tt.input)
		if got != tt.want {
			t.Errorf("marketLink(%q)\n  got  %q\n  want %q", tt.input, got, tt.want)
		}
	}
}

// --- min ---

func TestMin(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{3, 5, 3},
		{5, 3, 3},
		{0, 0, 0},
		{-1, 1, -1},
		{10, 10, 10},
	}
	for _, tt := range tests {
		if got := min(tt.a, tt.b); got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
