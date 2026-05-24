package steam

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	inventoryBaseURL = "https://steamcommunity.com/inventory"
	priceOverviewURL = "https://steamcommunity.com/market/priceoverview/"
)

// Sentinel errors checked by the bot layer for localised messages.
var (
	ErrInvalidSteamID = errors.New("invalid_steamid")
	ErrRateLimit      = errors.New("rate_limit")
	ErrPrivate        = errors.New("private")
)

// Currency holds a Steam market currency code and its display symbol.
type Currency struct {
	Code   int
	Symbol string
}

// KnownCurrencies maps ISO currency names to Steam API codes and symbols.
var KnownCurrencies = map[string]Currency{
	"USD": {1, "$"},
	"GBP": {2, "£"},
	"EUR": {3, "€"},
	"PLN": {6, "zł"},
	"RUB": {5, "₽"},
	"UAH": {18, "₴"},
	"KZT": {37, "₸"},
}

// Item represents a CS2 inventory item (may aggregate multiple assets of the same type).
type Item struct {
	MarketHashName string
	Name           string
	Type           string
	Amount         int
	Tradable       bool
	Marketable     bool
	Rarity         string   // rarity emoji: ⬜🟦🔵🟣🔴🟡🔶
	Locked         bool     // in trade lock (tradable=0, marketable=1)
	Stickers       []string // sticker names applied (from first asset)
	Charm          string   // charm name, if any
	NameTag        string   // custom name tag
	InspectLink    string   // steam://rungame inspect link for first asset
}

type Client struct {
	http          *http.Client
	tick          <-chan time.Time
	sessionCookie string
	Currency      Currency
}

func NewClient(sessionCookie string, currency Currency) *Client {
	jar, _ := cookiejar.New(nil)
	if sessionCookie != "" {
		steamURL, _ := url.Parse("https://steamcommunity.com")
		jar.SetCookies(steamURL, []*http.Cookie{
			{Name: "steamLoginSecure", Value: sessionCookie},
		})
	}

	ch := make(chan time.Time, 1)
	ch <- time.Now()
	go func() {
		for t := range time.Tick(1500 * time.Millisecond) {
			ch <- t
		}
	}()
	return &Client{
		http:          &http.Client{Timeout: 30 * time.Second, Jar: jar},
		tick:          ch,
		sessionCookie: sessionCookie,
		Currency:      currency,
	}
}

func (c *Client) wait() { <-c.tick }

// --- JSON types for inventory response ---

type invAsset struct {
	ClassID    string `json:"classid"`
	InstanceID string `json:"instanceid"`
	AssetID    string `json:"assetid"`
	Amount     string `json:"amount"`
}

type invTag struct {
	Category     string `json:"category"`
	InternalName string `json:"internal_name"`
}

type invDescEntry struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type invAction struct {
	Link string `json:"link"`
}

type invDesc struct {
	ClassID        string         `json:"classid"`
	InstanceID     string         `json:"instanceid"`
	MarketHashName string         `json:"market_hash_name"`
	Name           string         `json:"name"`
	Type           string         `json:"type"`
	Tradable       int            `json:"tradable"`
	Marketable     int            `json:"marketable"`
	Descriptions   []invDescEntry `json:"descriptions"`
	Tags           []invTag       `json:"tags"`
	Actions        []invAction    `json:"actions"`
}

type invPage struct {
	Assets       []invAsset `json:"assets"`
	Descriptions []invDesc  `json:"descriptions"`
	Success      int        `json:"success"`
	MoreItems    int        `json:"more_items"`
	LastAssetID  string     `json:"last_assetid"`
}

// FetchInventory fetches the full CS2 inventory (appid=730, context=2) with pagination.
func (c *Client) FetchInventory(steamID string) ([]Item, error) {
	if !isValidSteamID(steamID) {
		return nil, ErrInvalidSteamID
	}

	referer := "https://steamcommunity.com/profiles/" + steamID + "/inventory/"
	var allAssets []invAsset
	descMap := make(map[string]invDesc) // classid:instanceid → description
	startAssetID := ""

	for fetch := 0; fetch < 20; fetch++ { // safety cap: 20 pages × 2000 = 40 000 items
		u := fmt.Sprintf("%s/%s/730/2?l=english&count=2000", inventoryBaseURL, steamID)
		if startAssetID != "" {
			u += "&start_assetid=" + startAssetID
		}

		c.wait()
		resp, err := c.doGet(u, referer)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := strings.TrimSpace(string(body))

		if resp.StatusCode != 200 {
			preview := bodyStr
			if len(preview) > 200 {
				preview = preview[:200]
			}
			switch resp.StatusCode {
			case 400, 403:
				if preview == "null" || preview == "" {
					return nil, ErrPrivate
				}
				return nil, fmt.Errorf("Steam %d: %s", resp.StatusCode, preview)
			case 429:
				return nil, ErrRateLimit
			default:
				return nil, fmt.Errorf("Steam HTTP %d: %s", resp.StatusCode, preview)
			}
		}

		var pg invPage
		if err := json.Unmarshal([]byte(bodyStr), &pg); err != nil {
			return nil, fmt.Errorf("parse error: %w", err)
		}
		if pg.Success == 0 {
			return nil, ErrPrivate
		}

		allAssets = append(allAssets, pg.Assets...)
		for _, d := range pg.Descriptions {
			key := d.ClassID + ":" + d.InstanceID
			if _, exists := descMap[key]; !exists {
				descMap[key] = d
			}
		}

		if pg.MoreItems == 0 || pg.LastAssetID == "" {
			break
		}
		startAssetID = pg.LastAssetID
	}

	// Build deduplicated item map (aggregate by MarketHashName).
	itemMap := make(map[string]*Item)
	for _, a := range allAssets {
		key := a.ClassID + ":" + a.InstanceID
		d, ok := descMap[key]
		if !ok {
			continue
		}
		amount, _ := strconv.Atoi(a.Amount)
		if amount < 1 {
			amount = 1
		}

		if existing, ok := itemMap[d.MarketHashName]; ok {
			existing.Amount += amount
			if d.Tradable == 0 && d.Marketable == 1 {
				existing.Locked = true
			}
		} else {
			stickers, charm, nameTag := parseDescEntries(d.Descriptions)
			var inspectLink string
			if len(d.Actions) > 0 {
				lnk := d.Actions[0].Link
				lnk = strings.ReplaceAll(lnk, "%owner_steamid%", steamID)
				lnk = strings.ReplaceAll(lnk, "%assetid%", a.AssetID)
				inspectLink = lnk
			}
			itemMap[d.MarketHashName] = &Item{
				MarketHashName: d.MarketHashName,
				Name:           d.Name,
				Type:           d.Type,
				Amount:         amount,
				Tradable:       d.Tradable == 1,
				Marketable:     d.Marketable == 1,
				Rarity:         parseRarity(d.Name, d.Tags),
				Locked:         d.Tradable == 0 && d.Marketable == 1,
				Stickers:       stickers,
				Charm:          charm,
				NameTag:        nameTag,
				InspectLink:    inspectLink,
			}
		}
	}

	items := make([]Item, 0, len(itemMap))
	for _, it := range itemMap {
		items = append(items, *it)
	}
	return items, nil
}

// FetchPriceWithRetry wraps FetchPrice with automatic back-off on ErrRateLimit.
// It retries up to maxRetries additional times, sleeping 5 * attempt seconds between each.
// Non-rate-limit errors are returned immediately without retrying.
func (c *Client) FetchPriceWithRetry(marketHashName string, currencyCode, maxRetries int) (float64, error) {
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var price float64
		price, err = c.FetchPrice(marketHashName, currencyCode)
		if err == nil {
			return price, nil
		}
		if !errors.Is(err, ErrRateLimit) || attempt == maxRetries {
			return 0, err
		}
		time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
	}
	return 0, err
}

// FetchPrice returns the market price for an item.
// currencyCode 0 uses the client's default currency.
func (c *Client) FetchPrice(marketHashName string, currencyCode int) (float64, error) {
	if currencyCode == 0 {
		currencyCode = c.Currency.Code
	}
	params := url.Values{}
	params.Set("appid", "730")
	params.Set("currency", strconv.Itoa(currencyCode))
	params.Set("market_hash_name", marketHashName)

	c.wait()
	resp, err := c.doGet(priceOverviewURL+"?"+params.Encode(), "https://steamcommunity.com/market/")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return 0, ErrRateLimit
	}
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var pr struct {
		Success     bool   `json:"success"`
		LowestPrice string `json:"lowest_price"`
		MedianPrice string `json:"median_price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}
	if !pr.Success {
		return 0, nil
	}

	price := parsePrice(pr.MedianPrice)
	if price == 0 {
		price = parsePrice(pr.LowestPrice)
	}
	return price, nil
}

func (c *Client) doGet(u, referer string) (*http.Response, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return c.http.Do(req)
}

func isValidSteamID(s string) bool {
	if len(s) != 17 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return strings.HasPrefix(s, "7656119")
}

// --- Inventory parsing helpers ---

// rarityEmojis maps Steam rarity keys to coloured circle emoji.
// Rarity_Ancient is handled separately in parseRarity (knives/gloves vs covert skins).
var rarityEmojis = map[string]string{
	"Rarity_Common":     "⬜", // Consumer Grade   – white
	"Rarity_Uncommon":   "🩵", // Industrial Grade – light-blue
	"Rarity_Rare":       "🔵", // Mil-Spec          – blue
	"Rarity_Mythical":   "🟣", // Restricted        – purple
	"Rarity_Legendary":  "🩷", // Classified        – pink
	"Rarity_Contraband": "⭐", // Contraband        – gold star (only M4A4 | Howl)
	// Rarity_Ancient → see parseRarity below
}

// parseRarity returns a rarity emoji for the item.
// name is used to distinguish knives/gloves (name starts with ★, shown as 🟡)
// from regular Covert skins (🔴).
func parseRarity(name string, tags []invTag) string {
	for _, t := range tags {
		if t.Category != "Rarity" {
			continue
		}
		key := t.InternalName
		for _, suffix := range []string{"_Weapon", "_Character", "_Equipment"} {
			key = strings.TrimSuffix(key, suffix)
		}
		if key == "Rarity_Ancient" {
			// Knives and gloves always have ★ at the start of their name.
			if strings.HasPrefix(name, "★") {
				return "🟡" // knife / gloves – yellow
			}
			return "🔴" // covert skin – red
		}
		if emoji, ok := rarityEmojis[key]; ok {
			return emoji
		}
	}
	return ""
}

func parseDescEntries(entries []invDescEntry) (stickers []string, charm, nameTag string) {
	for _, e := range entries {
		plain := strings.TrimSpace(stripHTML(e.Value))
		switch {
		case strings.HasPrefix(plain, "Sticker:"):
			for _, s := range strings.Split(strings.TrimSpace(plain[len("Sticker:"):]), ",") {
				if s = strings.TrimSpace(s); s != "" {
					stickers = append(stickers, s)
				}
			}
		case strings.HasPrefix(plain, "Charm:"):
			charm = strings.TrimSpace(plain[len("Charm:"):])
		case strings.HasPrefix(plain, "Name Tag:"):
			nameTag = strings.Trim(strings.TrimSpace(plain[len("Name Tag:"):]), "'\"")
		}
	}
	return
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// parsePrice handles both US ("1,234.56") and European ("1.234,56") decimal formats.
func parsePrice(s string) float64 {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' || r == ',' || r == '.' {
			b.WriteRune(r)
		}
	}
	clean := b.String()
	if clean == "" {
		return 0
	}

	lastDot := strings.LastIndex(clean, ".")
	lastComma := strings.LastIndex(clean, ",")

	if lastComma > lastDot {
		clean = strings.ReplaceAll(clean, ".", "")
		clean = strings.ReplaceAll(clean, ",", ".")
	} else {
		clean = strings.ReplaceAll(clean, ",", "")
	}

	v, _ := strconv.ParseFloat(clean, 64)
	return v
}
