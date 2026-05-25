package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/slain505/steam-price-bot/internal/bot"
	"github.com/slain505/steam-price-bot/internal/pricecache"
	"github.com/slain505/steam-price-bot/internal/reporter"
	"github.com/slain505/steam-price-bot/internal/steam"
	"github.com/slain505/steam-price-bot/internal/storage"
	"github.com/slain505/steam-price-bot/internal/tracker"
)

func main() {
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load(".env.example")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "prices.db"
	}

	sessionCookie := os.Getenv("STEAM_SESSION_COOKIE")
	if sessionCookie == "" {
		log.Println("WARNING: STEAM_SESSION_COOKIE not set")
	} else {
		log.Println("Steam session cookie: OK")
	}

	currencyCode := strings.ToUpper(os.Getenv("STEAM_CURRENCY"))
	currency, ok := steam.KnownCurrencies[currencyCode]
	if !ok {
		currency = steam.KnownCurrencies["USD"]
		log.Printf("Currency: USD (supported: USD, EUR, RUB, GBP, UAH, KZT)")
	} else {
		log.Printf("Currency: %s (%s)", currencyCode, currency.Symbol)
	}

	store, err := storage.New(dbPath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	defer store.Close()

	// Backfill: enroll every existing account into the tracker. Older versions
	// of the bot kept accounts and tracking as independent concepts, so users
	// from those installs may have accounts that were never explicitly tracked.
	if n, err := store.BackfillTrackingFromAccounts(); err != nil {
		log.Printf("backfill tracking: %v", err)
	} else if n > 0 {
		log.Printf("backfill tracking: enrolled %d previously-untracked accounts", n)
	}

	steamClient := steam.NewClient(sessionCookie, currency)

	// Skinport bulk price cache — only pre-load the bot's configured currency.
	// Loading all currencies at once causes HTTP 429 from Skinport.
	// RUB/KZT/UAH are not supported by Skinport and are silently skipped.
	pc := pricecache.New()
	skinportSupported := map[string]bool{"USD": true, "EUR": true, "GBP": true, "PLN": true}
	if skinportSupported[currencyCode] {
		pc.StartAutoRefresh([]string{currencyCode})
	} else {
		pc.StartAutoRefresh(nil) // unsupported currency — cache stays empty, Steam API used instead
	}
	defer pc.Stop()

	tr := tracker.New(store, steamClient, pc)

	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_USER_ID"), 10, 64)
	if adminID != 0 {
		log.Printf("Admin user ID: %d", adminID)
	}

	b, err := bot.New(token, store, steamClient, tr, pc, adminID)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	// Pipe operational warnings (stale cookie, etc.) into the admin chat.
	tr.SetAdminNotifyFunc(b.NotifyAdmin)

	tr.Start()
	defer tr.Stop()

	rep := reporter.New(store, b.SendAlert)
	rep.Start()
	defer rep.Stop()

	log.Println("Bot started. Press Ctrl+C to stop.")
	b.Start()
}
