# 🎮 CS2 Price Tracker — Telegram Bot

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![Telegram Bot API](https://img.shields.io/badge/Telegram%20Bot%20API-v5-26A5E4?logo=telegram)](https://core.telegram.org/bots/api)

A self-hosted Telegram bot that tracks your **CS2 inventory value**, monitors price changes over time, manages portfolio P&L, and fires instant alerts — all powered by a local SQLite cache so repeated lookups cost zero API calls.

---

## ✨ Features

### 📦 Inventory
- Top items sorted by value with full context: **rarity colour**, wear grade, StatTrak™, trade lock status
- **Item detail card** — tap any item number to open a card with applied stickers, charm, name tag, Steam Market link, and in-game Inspect link
- Supports inventories with 2 000 + items via automatic pagination

### 💰 Portfolio
- Total value with **breakdown by rarity tier**
- **P&L vs entry price** — see how much each item has gained or lost since you bought it
- Gainers / Losers top-10 across any time window

### ✏️ Entry Price Management
- On first scan the bot records the current market price as your entry price automatically
- **Paginated editor** (8 items/page) lets you correct any price to match what you actually paid on the marketplace
- **Per-item edit button** directly from the inventory list — no need to navigate away

### 📊 Price History
- 24 h / 7 d / 30 d price change view per inventory
- Top movers (biggest gainers and losers)
- History built passively in the background — no manual action needed

### 🔔 Alerts
- Custom alerts: notify when an item moves **±N%** from a saved baseline
- **Auto spike alerts**: instant notification on any ≥+5% jump detected during the hourly refresh

### ⚙️ Background Tracker
- Hourly auto-refresh of tracked inventories
- Prices stored in SQLite — every command reads from cache, no live API call overhead

### 📅 Daily Report
- Scheduled portfolio summary (total value, P&L, biggest movers in 24 h)
- Each user picks their own UTC hour

### 📥 CSV Export
- Full inventory dump with current price, entry price, P&L amount and P&L %

### 🌍 Internationalisation
| Language | Code |
|---|---|
| 🇬🇧 English | `en` |
| 🇷🇺 Russian | `ru` |
| 🇺🇦 Ukrainian | `ua` |
| 🇰🇿 Kazakh | `kz` |
| 🇵🇱 Polish | `pl` |
| 🇩🇪 German | `de` |

### 💱 Supported Currencies
`USD` · `EUR` · `GBP` · `PLN` · `UAH` · `RUB` · `KZT`

---

## 🏎️ Pricing Strategy

The bot uses a **three-tier** approach to get prices as fast as possible while minimising external API calls:

```
┌─────────────────────────────────────────────────────┐
│  1. Local DB cache   (instant, 0 API calls)          │  ← already seen this item
│  2. Skinport bulk    (one HTTP request for ALL items) │  ← first scan, known currencies
│  3. Steam Market     (per-item fallback)              │  ← RUB / KZT / item not on Skinport
└─────────────────────────────────────────────────────┘
```

| Source | Detail |
|---|---|
| **SQLite cache** | Every known item is priced from the local DB — no network I/O |
| **Skinport API** | `api.skinport.com/v1/items` — fetches ~50 000 CS2 items in **one brotli-compressed request** every 6 h; covers USD, EUR, GBP, PLN, UAH |
| **Steam Market API** | Per-item fallback for RUB, KZT or items absent from Skinport; rate-limited to 1 req / 1.5 s with automatic retry on HTTP 429 |

First scan of a 200-item inventory typically completes in **< 5 seconds** (Skinport bulk).  
Every subsequent scan is **instant** (DB cache).

---

## 🚀 Quick Start (Docker)

```bash
# 1. Clone the repository
git clone https://github.com/Slain505/steam_price_bot.git
cd steam_price_bot

# 2. Create your .env file
cp .env.example .env
# then open .env and fill in TELEGRAM_BOT_TOKEN and optionally STEAM_SESSION_COOKIE

# 3. Start
docker compose up -d
```

The SQLite database lives in a named Docker volume (`bot_data`) and survives container rebuilds.

### Update after code changes

```bash
docker compose up -d --build
```

### View logs

```bash
docker compose logs -f
```

---

## 🛠️ Local Development

**Requirements:** Go 1.22+

```bash
# Run directly
go run ./cmd/bot

# Or build a binary
go build -o steam-price-bot ./cmd/bot
./steam-price-bot
```

---

## ⚙️ Configuration

Copy `.env.example` to `.env` and edit:

```env
# ── Required ────────────────────────────────────────────
# Telegram bot token — get it from @BotFather
TELEGRAM_BOT_TOKEN=123456:ABC-your-token-here

# ── Recommended ─────────────────────────────────────────
# Needed to read private inventories.
# How to get: log in at steamcommunity.com → F12 →
#   Application → Cookies → steamcommunity.com →
#   copy the value of "steamLoginSecure"
STEAM_SESSION_COOKIE=

# ── Optional ────────────────────────────────────────────
# Default display currency (USD if omitted)
# Supported: USD EUR GBP PLN UAH RUB KZT
STEAM_CURRENCY=USD

# Path to the SQLite database file (default: prices.db)
# Docker compose overrides this to /data/prices.db automatically
DB_PATH=prices.db
```

---

## 📖 Bot Commands

| Command | Description |
|---|---|
| `/start` | Onboarding: choose language → add first Steam account |
| `/inventory [steamid64]` | Show top items by value with P&L badges |
| `/value [steamid64]` | Portfolio total + rarity breakdown + overall P&L |
| `/changes [steamid64] [24h\|7d\|30d]` | Price change history for your items |
| `/top [steamid64] [gainers\|losers] [period]` | Biggest movers over a time window |
| `/track <steamid64>` | Start hourly auto-refresh for an inventory |
| `/untrack <steamid64>` | Stop tracking |
| `/status` | List all tracked inventories |
| `/alert add <item name> <percent>` | Add a ±% price alert |
| `/alert list` | List your active alerts |
| `/setentry <item name> <price>` | Manually set an entry price |
| `/export [steamid64]` | Download full inventory as a `.csv` file |
| `/currency` | Change your display currency |
| `/pnl` | Portfolio P&L overview (top gainers / losers) |
| `/cancel` | Cancel any in-progress input |

All commands are also reachable via the **reply-keyboard menu** — no need to memorise them.

---

## 🗂️ Project Structure

```
steam_price_bot/
├── cmd/
│   └── bot/
│       └── main.go              # Entry point, wires everything together
├── internal/
│   ├── bot/
│   │   ├── handler.go           # All Telegram handlers, callbacks, UI helpers
│   │   └── state.go             # In-memory per-user state machine
│   ├── steam/
│   │   └── client.go            # Steam inventory & market price API client
│   ├── storage/
│   │   └── storage.go           # SQLite persistence layer
│   ├── tracker/
│   │   └── tracker.go           # Background hourly refresh + spike alerts
│   ├── reporter/
│   │   └── reporter.go          # Daily portfolio report scheduler
│   ├── pricecache/
│   │   └── pricecache.go        # Skinport bulk price cache (refreshed every 6 h)
│   └── i18n/
│       └── i18n.go              # Translations for 6 languages
├── Dockerfile
├── docker-compose.yml
├── .env.example
└── go.mod
```

---

## 🔩 Architecture Notes

**No CGo** — uses `modernc.org/sqlite` (pure Go), so the binary cross-compiles with no native dependencies.

**Stateless restarts** — on startup the bot drains any accumulated Telegram updates (short-poll with timeout=0) so stale messages from downtime are never re-processed.

**Rate-limit handling** — Steam Market API calls are paced at 1 req / 1.5 s via a buffered ticker channel. On HTTP 429 the bot automatically backs off and retries (up to 3× with 5 s / 10 s / 15 s delays).

**Brotli decompression** — Skinport requires `Accept-Encoding: br`. The response is decoded using `github.com/andybalholm/brotli` before JSON parsing.

**Deterministic item ordering** — all paginated views (inventory list, entry price editor) sort by `price × amount` descending then by `market_hash_name` ascending, so button indices always point to the correct item even across page navigations.

---

## 🤝 Contributing

Pull requests are welcome. For major changes please open an issue first to discuss what you'd like to change.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

[MIT](LICENSE) © 2024
