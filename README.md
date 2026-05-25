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
- **30-minute heartbeat digest**: every half hour each user gets one consolidated message listing every alert and its current state (price, delta, threshold, triggered/within-threshold). Proves the pipeline is alive even when nothing has moved enough to fire.

### ⚙️ Background Tracker
- Hourly auto-refresh of tracked inventories
- Prices stored in SQLite — every command reads from cache, no live API call overhead
- **Auto-tracking** — every account you add is enrolled in the tracker on creation. Older installs are caught up on first start via an idempotent backfill.

### 🛡 Operator Alerts
When `STEAM_SESSION_COOKIE` goes stale, Steam silently serves a partial inventory (e.g. 178 items instead of the real 226). The bot detects this and pings the configured `ADMIN_USER_ID` on Telegram with a one-line refresh instruction — once per affected steamID per 24 h so the chat isn't spammed.

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

# Your Telegram user ID. When set, grants access to the /admin
# panel (force tracker tick, fire test alerts, bot status) and
# receives operator alerts when STEAM_SESSION_COOKIE goes stale.
# Get your ID by messaging @userinfobot.
ADMIN_USER_ID=
```

> **About `STEAM_SESSION_COOKIE`**: Steam's `steamLoginSecure` is a short-lived JWT (~30 min) that the browser auto-rotates via a refresh token cookie. Because the bot only carries a snapshot, it works until Steam invalidates the underlying session — usually 1–3 months of normal use, or instantly after "Sign out everywhere". When that happens you'll get an admin alert in Telegram; just re-extract the cookie from your logged-in browser and restart.

---

## 📖 Bot Commands

| Command | Description |
|---|---|
| `/start` | Onboarding: choose language → add first Steam account |
| `/inventory [steamid64]` | Show top items by value with P&L badges |
| `/value [steamid64]` | Portfolio total + rarity breakdown + overall P&L |
| `/changes [steamid64] [24h\|7d\|30d]` | Price change history for your items |
| `/top [steamid64] [gainers\|losers] [period]` | Biggest movers over a time window |
| `/track <steamid64>` | Start hourly auto-refresh (rarely needed — new accounts are auto-tracked) |
| `/untrack <steamid64>` | Stop tracking an inventory while keeping the account |
| `/status` | List all tracked inventories |
| `/alert add <item name> <percent>` | Add a ±% price alert |
| `/alert list` | List your active alerts |
| `/setentry <item name> <price>` | Manually set an entry price |
| `/export [steamid64]` | Download full inventory as a `.csv` file |
| `/currency` | Change your display currency |
| `/pnl` | Portfolio P&L overview (top gainers / losers) |
| `/admin` | Admin panel (requires `ADMIN_USER_ID`) |
| `/cancel` | Cancel any in-progress input |

All commands are also reachable via the **reply-keyboard menu** — no need to memorise them.

### Admin panel

Set `ADMIN_USER_ID` in `.env` to your Telegram user ID. The `/admin` command then unlocks:

- **🔄 Force Refresh** — triggers an immediate tracker tick instead of waiting up to an hour
- **🔔 Test Alerts** — fires every configured alert ignoring thresholds, useful to verify the alert delivery pipeline end-to-end
- **📊 Bot Status** — uptime, last tracker run, unique items priced, tracked accounts, cached inventories

The same admin chat also receives the automatic **cookie-stale alerts** described above.

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

**Stateless restarts** — on startup the bot drains any accumulated Telegram updates (short-poll with timeout=0) so stale messages from downtime are never re-processed. The persisted `tracker_last_run` timestamp survives restarts, so the hourly schedule isn't reset when the container reboots.

**Steam Market rate limiting** — Steam Market API calls are paced at **1 req / 4 s** via a buffered ticker channel (Steam stopped tolerating 1.5 s and started 429-ing aggressively). User-triggered `🔄 Refresh` is additionally throttled at **once per 60 s per user** so spam-clicks can't pile bulk fetches on top of each other.

**Inventory cache workaround** — Steam serves cached inventory snapshots keyed by the requesting session. The bot mitigates this with: a warmup GET to `/profiles/{steamid}/inventory/` before each fetch, cache-busting query/headers, and browser-like cookies (`sessionid`, `browserid`, `steamCountry`, `Steam_Language`). When the snapshot still looks truncated (< 20 items), the admin gets pinged. See the "Operator Alerts" section.

**Marketable aggregation** — when two copies of the same `market_hash_name` carry different `tradable`/`marketable` flags (e.g. one in 7-day market cooldown after a recent purchase), the bot OR-combines them rather than taking the first asset iterated. This prevents whole stacks from silently disappearing.

**Brotli decompression** — Skinport requires `Accept-Encoding: br`. The response is decoded using `github.com/andybalholm/brotli` before JSON parsing.

**Skinport cache** — bulk price catalogue is loaded **synchronously** on startup so the cache is warm before the first user request. The configured `STEAM_CURRENCY` decides which currency to fetch (USD/EUR/GBP/PLN; RUB/UAH/KZT fall back to Steam Market). Retries on HTTP 429 are capped at 10 s + 20 s to avoid blocking startup for too long.

**Deterministic item ordering** — all paginated views (inventory list, entry price editor) sort by `price × amount` descending then by `market_hash_name` ascending, so button indices always point to the correct item even across page navigations.

---

## ✅ Manual Test Plan

Use these checks after a fresh deploy or material change. The unit suite (`go test -race ./...`) covers the unit-level invariants; this list covers the integration paths that need a real Steam account.

**Startup**
- [ ] Container starts and logs `pricecache: initial load complete` within ~30 s
- [ ] `backfill tracking: enrolled N previously-untracked accounts` appears on the first start after upgrade (subsequent starts are silent — backfill is idempotent)
- [ ] `tracker: last run X ago, next tick in Y` is logged, showing the hourly schedule survived the restart

**Inventory accuracy**
- [ ] `/inventory` for a fresh-cookie account returns the full item set (not a truncated subset)
- [ ] Log line `steam: inventory ... — N assets → K unique types (X marketable, Y non-marketable)` matches what the Steam Market UI shows
- [ ] An account whose cookie has gone stale produces a Telegram message to `ADMIN_USER_ID` describing the issue and the F12 cookie refresh steps

**Auto-tracking**
- [ ] Add a new Steam account through the bot UI → it appears on the next `tracker: tick` log line without a manual `/track` call
- [ ] Remove an account → it disappears from both `accounts` and `tracked_inventories` tables

**Alerts pipeline**
- [ ] Add a `/alert` for an item that is unlikely to move much
- [ ] Within 30 minutes a `📊 30-min Alert Check` digest arrives in Telegram listing that alert with the current price, delta, threshold, and the appropriate icon (`⏸`, `🔹`, `🔸`, `📈`, `📉`)
- [ ] Admin `/admin → 🔔 Test Alerts` immediately fires a test message for every configured alert, ignoring thresholds

**Refresh throttling**
- [ ] Tap the `🔄 Refresh` button on `/inventory` twice in quick succession → the second tap is rejected with a "wait N seconds" message

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
