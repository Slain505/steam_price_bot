package reporter

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/slain505/steam-price-bot/internal/i18n"
	"github.com/slain505/steam-price-bot/internal/storage"
)

// SendFunc delivers an HTML message to a Telegram user.
type SendFunc func(userID int64, html string)

// Reporter sends scheduled daily portfolio reports to users.
type Reporter struct {
	store *storage.Storage
	send  SendFunc
	quit  chan struct{}
}

func New(s *storage.Storage, send SendFunc) *Reporter {
	return &Reporter{store: s, send: send, quit: make(chan struct{})}
}

func (r *Reporter) Start() { go r.loop() }
func (r *Reporter) Stop()  { close(r.quit) }

func (r *Reporter) loop() {
	// Wait until the start of the next minute, then tick every minute.
	now := time.Now().UTC()
	wait := time.Duration(60-now.Second()) * time.Second
	select {
	case <-time.After(wait):
	case <-r.quit:
		return
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	// Check immediately on first aligned tick.
	r.check()
	for {
		select {
		case <-ticker.C:
			r.check()
		case <-r.quit:
			return
		}
	}
}

func (r *Reporter) check() {
	now := time.Now().UTC()
	// Only fire at :00 of each hour.
	if now.Minute() != 0 {
		return
	}
	users, err := r.store.GetReportUsers(now.Hour())
	if err != nil {
		log.Printf("reporter: get users for hour %d: %v", now.Hour(), err)
		return
	}
	for _, userID := range users {
		msg, err := r.build(userID)
		if err != nil {
			log.Printf("reporter: build for user %d: %v", userID, err)
			continue
		}
		r.send(userID, msg)
	}
}

func (r *Reporter) build(userID int64) (string, error) {
	lang := i18n.FromString(r.store.GetLang(userID))
	msgs := i18n.Get(lang)

	currencyCode := r.store.GetCurrency(userID)
	sym := currencySymbol(currencyCode)

	records, err := r.store.GetAllEntryPrices(userID)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return msgs.ReportHeader + "\n\n" + msgs.ReportNoData, nil
	}

	type item struct {
		name    string
		entry   float64
		current float64
		pct     float64
	}

	var items []item
	var totalCurrent, totalEntry float64
	past24 := time.Now().UTC().Add(-24 * time.Hour)

	for _, rec := range records {
		cur, _, lerr := r.store.GetLatestPrice(rec.MarketHashName)
		if lerr != nil || cur == 0 {
			continue
		}
		prev, _ := r.store.GetPriceAt(rec.MarketHashName, past24)
		pct := 0.0
		if prev > 0 {
			pct = (cur - prev) / prev * 100
		}
		items = append(items, item{rec.MarketHashName, rec.EntryPrice, cur, pct})
		totalCurrent += cur
		totalEntry += rec.EntryPrice
	}

	if len(items) == 0 {
		return msgs.ReportHeader + "\n\n" + msgs.ReportNoData, nil
	}

	// Sort by |pct| descending for movers section.
	sorted := make([]item, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].pct) > math.Abs(sorted[j].pct)
	})

	var sb strings.Builder
	sb.WriteString(msgs.ReportHeader + "\n\n")

	// Portfolio total
	fmt.Fprintf(&sb, msgs.ReportPortfolio+"\n", sym, totalCurrent)
	if totalEntry > 0 {
		pnl := totalCurrent - totalEntry
		pnlPct := pnl / totalEntry * 100
		fmt.Fprintf(&sb, msgs.ReportPnl+"\n", sym, pnl, pnlPct)
	}

	// Top movers (max 5, only if |pct| >= 1%)
	var movers []item
	for _, it := range sorted {
		if math.Abs(it.pct) < 1 {
			break
		}
		movers = append(movers, it)
		if len(movers) == 5 {
			break
		}
	}
	if len(movers) > 0 {
		sb.WriteString("\n" + msgs.ReportMovers + "\n")
		for _, it := range movers {
			arrow := "📈"
			if it.pct < 0 {
				arrow = "📉"
			}
			mURL := "https://steamcommunity.com/market/listings/730/" + url.PathEscape(it.name)
			shortN := shortName(it.name)
			fmt.Fprintf(&sb, "%s <a href=\"%s\">%s</a> %+.1f%%\n",
				arrow, mURL, escHTML(shortN), it.pct)
		}
	}

	return sb.String(), nil
}

// currencySymbol returns the display symbol for a currency code.
func currencySymbol(code string) string {
	symbols := map[string]string{
		"USD": "$", "EUR": "€", "GBP": "£",
		"RUB": "₽", "UAH": "₴", "KZT": "₸", "PLN": "zł",
	}
	if s, ok := symbols[code]; ok {
		return s
	}
	return "$"
}

var qualityNames = []string{
	"Factory New", "Minimal Wear", "Field-Tested", "Well-Worn", "Battle-Scarred",
}

func shortName(name string) string {
	for _, q := range qualityNames {
		name = strings.TrimSuffix(name, " ("+q+")")
	}
	name = strings.TrimPrefix(name, "★ ")
	return strings.TrimSpace(name)
}

func escHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
