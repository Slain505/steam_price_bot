package main

import (
	"log"
	"os"
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

	steamClient := steam.NewClient(sessionCookie, currency)
	tr := tracker.New(store, steamClient)

	// Start Skinport bulk price cache. Covers USD, EUR, GBP, PLN, UAH.
	// RUB and KZT are not supported by Skinport and will fall back to Steam API.
	pc := pricecache.New()
	pc.StartAutoRefresh([]string{"USD", "EUR", "GBP", "PLN", "UAH"})
	defer pc.Stop()

	b, err := bot.New(token, store, steamClient, tr, pc)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	tr.Start()
	defer tr.Stop()

	rep := reporter.New(store, b.SendAlert)
	rep.Start()
	defer rep.Stop()

	log.Println("Bot started. Press Ctrl+C to stop.")
	b.Start()
}
