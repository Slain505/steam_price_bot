# CS2 Price Tracker Bot

A Telegram bot for tracking CS2 (Counter-Strike 2) inventory prices, portfolio value, P&L, and price alerts.

## Features

- **Inventory viewer** — top items sorted by value with rarity colour, wear, StatTrak™, stickers, charm and trade-lock indicators
- **Item detail card** — full info per item: price, P&L vs entry, applied stickers/charm/name-tag, Steam Market & Inspect links
- **Portfolio value** — total worth broken down by rarity, P&L vs entry prices
- **Price changes** — 24 h / 7 d / 30 d history with biggest movers
- **Entry price management** — paginated list of all items with editable purchase prices; also accessible directly from item detail card
- **Price alerts** — notify when any item moves ±N% from a baseline
- **Auto price spike alerts** — instant notification on ≥+5% jump detected during hourly tracker refresh
- **Inventory tracking** — background hourly refresh with price history stored in SQLite
- **Daily report** — scheduled portfolio summary at a chosen UTC hour
- **CSV export** — full inventory with current & entry prices and P&L
- **Multi-account** — manage several Steam accounts per Telegram user
- **6 languages** — English, Russian, Ukrainian, Kazakh, Polish, German
- **7 currencies** — USD, EUR, GBP, PLN, UAH, RUB, KZT

### Price data sources

| Source | Used for |
|---|---|
| **Skinport bulk API** | First scan of new items — one request fetches all ~50 000 CS2 items (USD/EUR/GBP/PLN/UAH) |
| **Steam Market API** | Per-item fallback for currencies not on Skinport (RUB, KZT) or items missing from Skinport |
| **Local DB cache** | All subsequent lookups — zero API calls for known items |

---

## Quick start with Docker

```bash
# 1. Clone
git clone https://github.com/slain505/steam-price-bot.git
cd steam-price-bot

# 2. Create .env (see section below)
cp .env.example .env
nano .env

# 3. Run
docker compose up -d
```

The database is stored in a named Docker volume (`bot_data`) so it survives container rebuilds.

### Rebuild after code changes

```bash
docker compose up -d --build
```

---

## Environment variables

Create a `.env` file in the project root:

```env
# Required
TELEGRAM_BOT_TOKEN=123456:ABC...     # from @BotFather

# Optional but recommended — enables private inventory access
STEAM_SESSION_COOKIE=<steamLoginSecure cookie value>

# Default currency shown in the bot (USD if omitted)
# Supported: USD EUR GBP PLN UAH RUB KZT
STEAM_CURRENCY=USD

# SQLite database path (default: prices.db)
# When running in Docker this is overridden to /data/prices.db by docker-compose.yml
DB_PATH=prices.db
```

> **How to get `steamLoginSecure`:** Log in to steamcommunity.com in your browser, open DevTools → Application → Cookies → find `steamLoginSecure`.

---

## Local development

Requirements: **Go 1.22+**

```bash
go run ./cmd/bot
```

Or build a binary:

```bash
go build -o steam-price-bot ./cmd/bot
./steam-price-bot
```

---

## Bot commands

| Command | Description |
|---|---|
| `/inventory [steamid64]` | Show top items by value |
| `/value [steamid64]` | Portfolio total with rarity breakdown |
| `/changes [steamid64] [24h\|7d\|30d]` | Price change history |
| `/top [steamid64] [gainers\|losers] [24h\|7d\|30d]` | Top movers |
| `/track <steamid64>` | Enable hourly auto-refresh for an inventory |
| `/untrack <steamid64>` | Stop tracking |
| `/status` | List tracked inventories |
| `/alert add <item> <%>` | Add a price alert |
| `/alert list` | List your alerts |
| `/setentry <item> <price>` | Manually override an entry price |
| `/export [steamid64]` | Download inventory as CSV |
| `/currency` | Change display currency |
| `/cancel` | Cancel current action |

---

## Project structure

```
cmd/bot/          — main entry point
internal/
  bot/            — Telegram handlers, state machine, UI helpers
  steam/          — Steam inventory & market price API client
  storage/        — SQLite persistence (prices, accounts, alerts, settings)
  tracker/        — Background hourly price refresh + spike alerts
  reporter/       — Daily portfolio report scheduler
  pricecache/     — Skinport bulk price cache (refreshed every 6 h)
  i18n/           — Translations for 6 languages
```

---

## Architecture notes

- **Pure Go SQLite** via `modernc.org/sqlite` — no CGo, builds anywhere
- **Rate-limit safe** — 1.5 s between Steam API calls; automatic retry with back-off on HTTP 429
- **DB-first pricing** — known items are priced from local DB (instant); only brand-new items hit external APIs
- **Brotli support** — Skinport API requires `Accept-Encoding: br`; handled via `github.com/andybalholm/brotli`
- **Stateless restarts** — pending Telegram updates flushed on startup so stale messages are never re-processed

---

## License

MIT
