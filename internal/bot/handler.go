package bot

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/slain505/steam-price-bot/internal/i18n"
	"github.com/slain505/steam-price-bot/internal/pricecache"
	"github.com/slain505/steam-price-bot/internal/steam"
	"github.com/slain505/steam-price-bot/internal/storage"
	"github.com/slain505/steam-price-bot/internal/tracker"
)

type Bot struct {
	api     *tgbotapi.BotAPI
	store   *storage.Storage
	steam   *steam.Client
	tracker *tracker.Tracker
	prices  *pricecache.Cache // bulk price cache fetched from Skinport
	states  *stateManager
	mu      sync.Mutex
	working map[int64]bool
}

func New(token string, s *storage.Storage, sc *steam.Client, tr *tracker.Tracker, pc *pricecache.Cache) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	log.Printf("bot: authorized as @%s", api.Self.UserName)
	b := &Bot{
		api:     api,
		store:   s,
		steam:   sc,
		tracker: tr,
		prices:  pc,
		states:  newStateManager(),
		working: make(map[int64]bool),
	}
	tr.SetAlertFunc(b.sendAlert)
	return b, nil
}

// flushPendingUpdates discards any Telegram updates that accumulated while the
// bot was offline, so stale messages are not re-processed after a restart.
func (b *Bot) flushPendingUpdates() {
	cfg := tgbotapi.UpdateConfig{Offset: 0, Limit: 100, Timeout: 0}
	var lastID int
	for {
		updates, err := b.api.GetUpdates(cfg)
		if err != nil || len(updates) == 0 {
			break
		}
		lastID = updates[len(updates)-1].UpdateID
		cfg.Offset = lastID + 1
		if len(updates) < 100 {
			break
		}
	}
	if lastID > 0 {
		log.Printf("bot: flushed pending updates (last id: %d)", lastID)
	}
}

func (b *Bot) Start() {
	b.flushPendingUpdates()
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	for update := range b.api.GetUpdatesChan(u) {
		if update.CallbackQuery != nil {
			go b.handleCallback(update.CallbackQuery)
		} else if update.Message != nil {
			if update.Message.IsCommand() {
				go b.dispatch(update.Message)
			} else if update.Message.Text != "" {
				go b.handleText(update.Message)
			}
		}
	}
}

func (b *Bot) getLang(userID int64) i18n.Lang {
	return i18n.FromString(b.store.GetLang(userID))
}

// --- Text message handler: state machine + menu buttons ---

func (b *Bot) handleText(m *tgbotapi.Message) {
	userID := m.From.ID
	chatID := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	msgs := i18n.Get(b.getLang(userID))

	st := b.states.get(userID)
	switch st.id {
	case stateAwaitSteamID:
		b.handleSteamIDInput(chatID, userID, text, st.onboarding, msgs)
		return
	case stateAwaitAccName:
		b.handleAccNameInput(chatID, userID, text, st.pendingSteamID, st.onboarding, msgs)
		return
	case stateAwaitEntryPrice:
		b.handleEntryPriceInput(chatID, userID, text, st, msgs)
		return
	}

	switch text {
	case msgs.MenuInventory:
		b.menuInventory(chatID, userID, msgs)
	case msgs.MenuAccounts:
		b.showAccounts(chatID, userID, msgs)
	case msgs.MenuAnalytics:
		b.cmdPnl(chatID, userID)
	case msgs.MenuAlerts:
		b.alertList(chatID, userID)
	case msgs.MenuSettings:
		b.showSettings(chatID, userID, msgs)
	case msgs.MenuHelp:
		b.replyHTML(chatID, msgs.HelpText, nil)
	}
}

// --- Commands dispatcher ---

func (b *Bot) dispatch(m *tgbotapi.Message) {
	chatID := m.Chat.ID
	userID := m.From.ID
	msgs := i18n.Get(b.getLang(userID))

	switch m.Command() {
	case "start":
		b.cmdStart(chatID, userID)
	case "help":
		b.replyHTML(chatID, msgs.HelpText, nil)
	case "cancel":
		b.states.clear(userID)
		b.replyHTML(chatID, msgs.Cancel, nil)
	case "accounts":
		b.showAccounts(chatID, userID, msgs)
	case "inventory":
		b.cmdInventory(chatID, userID, strings.TrimSpace(m.CommandArguments()), false)
	case "value":
		b.cmdValue(chatID, userID, strings.TrimSpace(m.CommandArguments()))
	case "changes":
		args := strings.Fields(m.CommandArguments())
		steamID, period := "", "24h"
		if len(args) > 0 {
			steamID = args[0]
		}
		if len(args) > 1 {
			period = args[1]
		}
		b.cmdChanges(chatID, userID, steamID, period)
	case "top":
		args := strings.Fields(m.CommandArguments())
		steamID, mode, period := "", "gainers", "24h"
		if len(args) > 0 {
			steamID = args[0]
		}
		for _, a := range args[1:] {
			switch a {
			case "gainers", "losers":
				mode = a
			case "24h", "7d", "30d":
				period = a
			}
		}
		b.cmdTop(chatID, userID, steamID, mode, period)
	case "track":
		b.cmdTrack(chatID, userID, strings.TrimSpace(m.CommandArguments()))
	case "untrack":
		b.cmdUntrack(chatID, userID, strings.TrimSpace(m.CommandArguments()))
	case "status":
		b.cmdStatus(chatID, userID)
	case "currency":
		b.cmdCurrency(chatID, userID)
	case "pnl":
		b.cmdPnl(chatID, userID)
	case "setentry":
		b.cmdSetEntry(chatID, userID, strings.TrimSpace(m.CommandArguments()))
	case "export":
		b.cmdExport(chatID, userID, strings.TrimSpace(m.CommandArguments()))
	case "alert":
		b.cmdAlert(m)
	}
}

// --- /start + onboarding ---

func (b *Bot) cmdStart(chatID, userID int64) {
	msgs := i18n.Get(b.getLang(userID))
	if b.store.IsOnboarded(userID) {
		b.sendMainMenu(chatID, userID, msgs.OnboardDone)
		return
	}
	b.replyHTML(chatID, msgs.Welcome, langKeyboard())
}

// --- Language selection ---

func langKeyboard() *tgbotapi.InlineKeyboardMarkup {
	langs := i18n.All
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(langs); i += 3 {
		var row []tgbotapi.InlineKeyboardButton
		for j := i; j < i+3 && j < len(langs); j++ {
			l := langs[j]
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				i18n.LangButton(l), "setlang:"+string(l),
			))
		}
		rows = append(rows, row)
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

func (b *Bot) setLang(chatID, userID int64, msgID int, langCode string) {
	lang := i18n.FromString(langCode)
	_ = b.store.SetLang(userID, string(lang))
	msgs := i18n.Get(lang)

	edit := tgbotapi.NewEditMessageText(chatID, msgID, msgs.LangSet)
	edit.ParseMode = tgbotapi.ModeHTML
	b.api.Send(edit)

	if !b.store.IsOnboarded(userID) {
		b.states.set(userID, userState{id: stateAwaitSteamID, onboarding: true})
		b.replyHTML(chatID, msgs.AddFirstAcc, nil)
	} else {
		b.sendMainMenu(chatID, userID, msgs.LangSet)
	}
}

// --- Account add: state machine handlers ---

func (b *Bot) handleSteamIDInput(chatID, userID int64, text string, onboarding bool, msgs i18n.M) {
	if !isValidSteamID64(text) {
		b.replyHTML(chatID, msgs.AccAskSteamID, nil)
		return
	}
	exists, _ := b.store.HasAccount(userID, text)
	if exists {
		b.replyHTML(chatID, msgs.AccAlreadyExists, nil)
		return
	}
	b.states.set(userID, userState{
		id:             stateAwaitAccName,
		pendingSteamID: text,
		onboarding:     onboarding,
	})
	b.replyHTML(chatID, msgs.AccAskName, nil)
}

func (b *Bot) handleAccNameInput(chatID, userID int64, name, steamID string, onboarding bool, msgs i18n.M) {
	name = strings.TrimSpace(name)
	if name == "" {
		b.replyHTML(chatID, msgs.AccAskName, nil)
		return
	}
	if err := b.store.AddAccount(userID, steamID, name); err != nil {
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
		return
	}
	b.states.clear(userID)
	if onboarding {
		_ = b.store.SetOnboarded(userID)
		b.sendMainMenu(chatID, userID, msgs.OnboardDone)
	} else {
		b.replyHTML(chatID, fmt.Sprintf(msgs.AccAdded, esc(name), esc(steamID)), nil)
		b.showAccounts(chatID, userID, msgs)
	}
}

// --- Accounts management ---

func (b *Bot) showAccounts(chatID, userID int64, msgs i18n.M) {
	accs, err := b.store.GetAccounts(userID)
	if err != nil {
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
		return
	}

	var sb strings.Builder
	sb.WriteString(msgs.AccListHeader + "\n\n")

	var rows [][]tgbotapi.InlineKeyboardButton
	if len(accs) == 0 {
		sb.WriteString(msgs.AccNone)
	} else {
		for _, acc := range accs {
			fmt.Fprintf(&sb, "• <b>%s</b> — <code>%s</code>\n", esc(acc.Name), acc.SteamID)
		}
		// One row per account: [👤 Name] [🗑]
		for _, acc := range accs {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👤 "+acc.Name, "profile:"+acc.SteamID),
				tgbotapi.NewInlineKeyboardButtonData("🗑", fmt.Sprintf("acc_del:%d", acc.ID)),
			))
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(msgs.AccAdd, "acc_add"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.replyHTML(chatID, sb.String(), &kb)
}

func (b *Bot) deleteAccount(chatID, userID, accID int64, msgs i18n.M) {
	accs, _ := b.store.GetAccounts(userID)
	var name string
	for _, a := range accs {
		if a.ID == accID {
			name = a.Name
			break
		}
	}
	if err := b.store.RemoveAccount(userID, accID); err != nil {
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
		return
	}
	b.replyHTML(chatID, fmt.Sprintf(msgs.AccRemoved, esc(name)), nil)
	b.showAccounts(chatID, userID, msgs)
}

func (b *Bot) menuInventory(chatID, userID int64, msgs i18n.M) {
	accs, _ := b.store.GetAccounts(userID)
	if len(accs) == 0 {
		kb := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(msgs.AccAdd, "acc_add"),
			),
		)
		b.replyHTML(chatID, msgs.AccNone, &kb)
		return
	}
	if len(accs) == 1 {
		b.cmdInventory(chatID, userID, accs[0].SteamID, false)
		return
	}
	kb := accSelectKeyboard(accs, "inventory")
	b.replyHTML(chatID, msgs.SelectAccount, kb)
}

func accSelectKeyboard(accs []storage.Account, action string) *tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for i, acc := range accs {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(acc.Name, action+":"+acc.SteamID))
		if len(row) == 2 || i == len(accs)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

// --- Settings ---

func (b *Bot) showSettings(chatID, userID int64, msgs i18n.M) {
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.SettingsLang, "settings_lang"),
			tgbotapi.NewInlineKeyboardButtonData(msgs.SettingsCurrency, "settings_currency"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.SettingsReport, "settings_report"),
		),
	)
	b.replyHTML(chatID, msgs.MenuSettings, &kb)
}

func reportHourKeyboard(current int) *tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for h := 0; h < 24; h++ {
		label := fmt.Sprintf("%02d:00", h)
		if h == current {
			label = "✓ " + label
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("setreport:%d", h)))
		if len(row) == 4 || h == 23 {
			rows = append(rows, row)
			row = nil
		}
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🚫 Disable", "setreport:off"),
	))
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

// --- Main menu ---

func (b *Bot) sendMainMenu(chatID, userID int64, text string) {
	msgs := i18n.Get(b.getLang(userID))
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	kb := mainMenuKeyboard(msgs)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send main menu: %v", err)
	}
}

func mainMenuKeyboard(msgs i18n.M) tgbotapi.ReplyKeyboardMarkup {
	kb := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(msgs.MenuInventory),
			tgbotapi.NewKeyboardButton(msgs.MenuAccounts),
			tgbotapi.NewKeyboardButton(msgs.MenuAnalytics),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(msgs.MenuAlerts),
			tgbotapi.NewKeyboardButton(msgs.MenuSettings),
			tgbotapi.NewKeyboardButton(msgs.MenuHelp),
		),
	)
	kb.ResizeKeyboard = true
	return kb
}

// --- Callback handler ---

func (b *Bot) handleCallback(cq *tgbotapi.CallbackQuery) {
	b.api.Send(tgbotapi.NewCallback(cq.ID, ""))

	parts := strings.Split(cq.Data, ":")
	action := parts[0]
	chatID := cq.Message.Chat.ID
	userID := cq.From.ID
	msgs := i18n.Get(b.getLang(userID))

	switch action {
	case "setlang":
		if len(parts) >= 2 {
			b.setLang(chatID, userID, cq.Message.MessageID, parts[1])
		}
	case "acc_add":
		b.states.set(userID, userState{id: stateAwaitSteamID})
		b.replyHTML(chatID, msgs.AccAskSteamID, nil)
	case "acc_del":
		if len(parts) >= 2 {
			id, _ := strconv.ParseInt(parts[1], 10, 64)
			b.deleteAccount(chatID, userID, id, msgs)
		}
	case "settings_lang":
		b.replyHTML(chatID, msgs.ChooseLang, langKeyboard())
	case "settings_currency":
		current := b.store.GetCurrency(userID)
		edit := tgbotapi.NewEditMessageText(chatID, cq.Message.MessageID,
			fmt.Sprintf(msgs.CurrencyCurrent, current))
		edit.ParseMode = tgbotapi.ModeHTML
		edit.ReplyMarkup = currencyKeyboard(current)
		b.api.Send(edit)
	case "settings_report":
		current := b.store.GetReportHour(userID)
		edit := tgbotapi.NewEditMessageText(chatID, cq.Message.MessageID, msgs.ReportChooseHour)
		edit.ParseMode = tgbotapi.ModeHTML
		edit.ReplyMarkup = reportHourKeyboard(current)
		b.api.Send(edit)
	case "setreport":
		if len(parts) >= 2 {
			if parts[1] == "off" {
				_ = b.store.SetReportHour(userID, -1)
				edit := tgbotapi.NewEditMessageText(chatID, cq.Message.MessageID, msgs.ReportDisabled)
				edit.ParseMode = tgbotapi.ModeHTML
				b.api.Send(edit)
			} else {
				hour, err := strconv.Atoi(parts[1])
				if err == nil && hour >= 0 && hour <= 23 {
					_ = b.store.SetReportHour(userID, hour)
					edit := tgbotapi.NewEditMessageText(chatID, cq.Message.MessageID,
						fmt.Sprintf(msgs.ReportEnabled, hour))
					edit.ParseMode = tgbotapi.ModeHTML
					edit.ReplyMarkup = reportHourKeyboard(hour)
					b.api.Send(edit)
				}
			}
		}
	case "inventory":
		if len(parts) >= 2 {
			b.cmdInventory(chatID, userID, parts[1], false)
		}
	case "inventory_force":
		if len(parts) >= 2 {
			b.cmdInventory(chatID, userID, parts[1], true)
		}
	case "value":
		if len(parts) >= 2 {
			b.cmdValue(chatID, userID, parts[1])
		}
	case "changes":
		period := "24h"
		if len(parts) >= 3 {
			period = parts[2]
		}
		if len(parts) >= 2 {
			b.cmdChanges(chatID, userID, parts[1], period)
		}
	case "top":
		mode, period := "gainers", "24h"
		if len(parts) >= 4 {
			mode, period = parts[2], parts[3]
		}
		if len(parts) >= 2 {
			b.cmdTop(chatID, userID, parts[1], mode, period)
		}
	case "track":
		if len(parts) >= 2 {
			b.cmdTrack(chatID, userID, parts[1])
		}
	case "setcurrency":
		if len(parts) >= 2 {
			b.setCurrency(chatID, userID, cq.Message.MessageID, parts[1])
		}
	case "export":
		if len(parts) >= 2 {
			b.cmdExport(chatID, userID, parts[1])
		}
	case "profile":
		if len(parts) >= 2 {
			b.cmdProfile(chatID, userID, parts[1])
		}
	case "entries":
		// entries:steamID:page
		if len(parts) >= 3 {
			page, _ := strconv.Atoi(parts[2])
			b.cmdEntryPrices(chatID, userID, parts[1], page, cq.Message.MessageID)
		}
	case "entry_edit":
		// entry_edit:steamID:page:idx
		if len(parts) >= 4 {
			page, _ := strconv.Atoi(parts[2])
			idx, _ := strconv.Atoi(parts[3])
			b.startEntryEdit(chatID, userID, cq.Message.MessageID, parts[1], page, idx)
		}
	case "entry_cancel":
		// entry_cancel:steamID:page — clear state and restore the entries page
		b.states.clear(userID)
		if len(parts) >= 3 {
			page, _ := strconv.Atoi(parts[2])
			b.cmdEntryPrices(chatID, userID, parts[1], page, cq.Message.MessageID)
		}
	case "entry_edit_d":
		// entry_edit_d:steamID:idx — set entry price triggered from item detail card
		if len(parts) >= 3 {
			idx, _ := strconv.Atoi(parts[2])
			b.startEntryEditFromDetail(chatID, userID, cq.Message.MessageID, parts[1], idx)
		}
	case "item_detail":
		// item_detail:steamID:idx — open detail card as new message
		if len(parts) >= 3 {
			idx, _ := strconv.Atoi(parts[2])
			b.cmdItemDetail(chatID, userID, parts[1], idx)
		}
	case "item_detail_restore":
		// item_detail_restore:steamID:idx — cancel edit, restore detail card in place
		b.states.clear(userID)
		if len(parts) >= 3 {
			idx, _ := strconv.Atoi(parts[2])
			b.cmdItemDetailEdit(chatID, userID, parts[1], idx, cq.Message.MessageID)
		}
	case "noop":
		// page-indicator button — do nothing
	}
}

// --- getUserCurrency ---

func (b *Bot) getUserCurrency(userID int64) steam.Currency {
	code := b.store.GetCurrency(userID)
	if c, ok := steam.KnownCurrencies[code]; ok {
		return c
	}
	return steam.KnownCurrencies["USD"]
}

// --- /currency ---

func (b *Bot) cmdCurrency(chatID, userID int64) {
	msgs := i18n.Get(b.getLang(userID))
	current := b.store.GetCurrency(userID)
	b.replyHTML(chatID, fmt.Sprintf(msgs.CurrencyCurrent, current), currencyKeyboard(current))
}

func (b *Bot) setCurrency(chatID int64, userID int64, msgID int, code string) {
	if _, ok := steam.KnownCurrencies[code]; !ok {
		return
	}
	if err := b.store.SetCurrency(userID, code); err != nil {
		msgs := i18n.Get(b.getLang(userID))
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	c := steam.KnownCurrencies[code]
	msgs := i18n.Get(b.getLang(userID))
	edit := tgbotapi.NewEditMessageText(chatID, msgID,
		fmt.Sprintf(msgs.CurrencySet, c.Symbol, code))
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = currencyKeyboard(code)
	b.api.Send(edit)
}

func currencyKeyboard(current string) *tgbotapi.InlineKeyboardMarkup {
	type entry struct{ code, label string }
	all := []entry{
		{"USD", "$ USD"}, {"EUR", "€ EUR"}, {"RUB", "₽ RUB"},
		{"GBP", "£ GBP"}, {"PLN", "zł PLN"}, {"UAH", "₴ UAH"},
		{"KZT", "₸ KZT"},
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for i, e := range all {
		label := e.label
		if e.code == current {
			label = "✓ " + label
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, "setcurrency:"+e.code))
		if len(row) == 3 || i == len(all)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	return &kb
}

// steamErrText maps steam sentinel errors to localised messages.
func steamErrText(err error, msgs i18n.M) string {
	switch {
	case errors.Is(err, steam.ErrRateLimit):
		return msgs.ErrRateLimit
	case errors.Is(err, steam.ErrPrivate):
		return msgs.ErrPrivate
	case errors.Is(err, steam.ErrInvalidSteamID):
		return msgs.ErrNoCS2
	default:
		return fmt.Sprintf(msgs.ErrGeneric, esc(err.Error()))
	}
}

// marketLink returns the Steam Market URL for a CS2 item.
func marketLink(marketHashName string) string {
	return "https://steamcommunity.com/market/listings/730/" + url.PathEscape(marketHashName)
}

// --- Inventory cache helper ---

const inventoryCacheTTL = 5 * time.Minute

// loadInventory returns the inventory for a steamID.
// If forceRefresh=false and cache is fresh (< 5 min), returns cached data instantly.
// fromCache=true means no live Steam call was made.
func (b *Bot) loadInventory(steamID string, forceRefresh bool) (items []steam.Item, fromCache bool, err error) {
	if !forceRefresh {
		data, cachedAt, cerr := b.store.GetInventoryCache(steamID)
		if cerr == nil && data != nil && time.Since(cachedAt) < inventoryCacheTTL {
			if jsonErr := json.Unmarshal(data, &items); jsonErr == nil {
				return items, true, nil
			}
		}
	}
	items, err = b.steam.FetchInventory(steamID)
	if err != nil {
		return nil, false, err
	}
	if data, jsonErr := json.Marshal(items); jsonErr == nil {
		_ = b.store.SaveInventoryCache(steamID, data)
	}
	return items, false, nil
}

// --- /inventory ---

func (b *Bot) cmdInventory(chatID int64, userID int64, steamID string, forceRefresh bool) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		b.menuInventory(chatID, userID, msgs)
		return
	}
	if !b.tryLock(userID) {
		b.sendText(chatID, msgs.WorkingBusy)
		return
	}
	defer b.unlock(userID)

	progressMsg, _ := b.api.Send(tgbotapi.NewMessage(chatID, msgs.LoadingInventory))

	items, fromCache, err := b.loadInventory(steamID, forceRefresh)
	if err != nil {
		b.editText(chatID, progressMsg.MessageID, steamErrText(err, msgs))
		return
	}
	marketable := filterMarketable(items)
	if len(marketable) == 0 {
		b.editText(chatID, progressMsg.MessageID, msgs.InventoryEmpty)
		return
	}

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol

	// Fetch entry prices that already existed BEFORE this scan.
	// Used to suppress P&L on first-time items.
	allNames := make([]string, len(marketable))
	for i, it := range marketable {
		allNames[i] = it.MarketHashName
	}
	preExistingEntryMap, _ := b.store.GetEntryPricesMap(userID, allNames)

	type pricedItem struct {
		item  steam.Item
		price float64
	}
	var priced []pricedItem
	var noPrice int // items for which no price was available

	if fromCache {
		// Fast path: prices come from DB — no Steam API calls.
		for _, it := range marketable {
			price, _, _ := b.store.GetLatestPrice(it.MarketHashName)
			if price > 0 {
				priced = append(priced, pricedItem{it, price})
			} else {
				noPrice++
			}
		}
	} else {
		// Slow path: fresh inventory from Steam.
		// DB-first: use cached prices for known items; only call Steam for brand-new items.
		type fetchJob struct{ item steam.Item }
		var toFetch []fetchJob
		for _, it := range marketable {
			price, _, _ := b.store.GetLatestPrice(it.MarketHashName)
			if price > 0 {
				priced = append(priced, pricedItem{it, price})
			} else {
				toFetch = append(toFetch, fetchJob{it})
			}
		}

		if len(toFetch) > 0 {
			cached := len(marketable) - len(toFetch)
			b.editText(chatID, progressMsg.MessageID,
				fmt.Sprintf(msgs.GettingPricesNew, len(toFetch), cached))

			// Step 1: Skinport bulk cache — resolve as many items as possible instantly.
			// This avoids per-item Steam API calls for brand-new items.
			if b.prices != nil {
				currCode := b.store.GetCurrency(userID)
				var stillToFetch []fetchJob
				for _, job := range toFetch {
					if spPrice := b.prices.Get(job.item.MarketHashName, currCode); spPrice > 0 {
						_ = b.store.SavePrice(job.item.MarketHashName, spPrice)
						_ = b.store.SetEntryPriceIfNew(userID, job.item.MarketHashName, spPrice)
						priced = append(priced, pricedItem{job.item, spPrice})
					} else {
						stillToFetch = append(stillToFetch, job)
					}
				}
				toFetch = stillToFetch
			}

			// Step 2: Items still without a price — fall back to Steam API one by one.
			for done, job := range toFetch {
				// Update progress on every item (or every 3 for large lists).
				step := 1
				if len(toFetch) > 30 {
					step = 3
				}
				if done%step == 0 {
					b.editText(chatID, progressMsg.MessageID,
						fmt.Sprintf(msgs.PricesProgress, done+1, len(toFetch)))
				}

				// Retry up to 3 times on rate limit.
				var price float64
				var fetchErr error
				for attempt := 0; attempt <= 3; attempt++ {
					price, fetchErr = b.steam.FetchPrice(job.item.MarketHashName, currency.Code)
					if fetchErr == nil || !errors.Is(fetchErr, steam.ErrRateLimit) {
						break
					}
					delay := (attempt + 1) * 5
					// Show progress + retry notice together.
					b.editText(chatID, progressMsg.MessageID,
						fmt.Sprintf(msgs.PricesRetrying, done+1, len(toFetch), delay))
					time.Sleep(time.Duration(delay) * time.Second)
				}
				if fetchErr != nil {
					log.Printf("price %s: %v", job.item.MarketHashName, fetchErr)
					noPrice++
					continue
				}
				if price > 0 {
					_ = b.store.SavePrice(job.item.MarketHashName, price)
					_ = b.store.SetEntryPriceIfNew(userID, job.item.MarketHashName, price)
					priced = append(priced, pricedItem{job.item, price})
				} else {
					noPrice++ // item exists but has no market listing
				}
			}
		}
	}

	if len(priced) == 0 {
		b.editText(chatID, progressMsg.MessageID, msgs.NoPrices)
		return
	}

	sort.Slice(priced, func(i, j int) bool {
		return priced[i].price*float64(priced[i].item.Amount) > priced[j].price*float64(priced[j].item.Amount)
	})

	const limit = 20
	topN := priced[:min(limit, len(priced))]

	var sb strings.Builder
	accName := b.store.GetAccountName(userID, steamID)
	fmt.Fprintf(&sb, msgs.InventoryHeader, esc(accName), min(limit, len(priced)))

	var topTotal float64
	for _, e := range topN {
		val := e.price * float64(e.item.Amount)
		topTotal += val
		badge := itemBadge(e.item.Name, e.item.Type, e.item.Locked)
		name := shortName(e.item.Name)
		mLink := marketLink(e.item.MarketHashName)

		var extras strings.Builder
		if n := len(e.item.Stickers); n > 0 {
			fmt.Fprintf(&extras, " 🧩%d", n)
		}
		if e.item.Charm != "" {
			extras.WriteString(" 🎀")
		}
		if e.item.NameTag != "" {
			extras.WriteString(" 📛")
		}

		// Show P&L only for items that had an entry price before this scan.
		if ep := preExistingEntryMap[e.item.MarketHashName]; ep > 0 && e.price > 0 {
			pct := (e.price - ep) / ep * 100
			if math.Abs(pct) >= 1 {
				if pct > 0 {
					fmt.Fprintf(&extras, " 📈+%.0f%%", pct)
				} else {
					fmt.Fprintf(&extras, " 📉%.0f%%", pct)
				}
			}
		}

		linkedName := fmt.Sprintf(`<a href="%s">%s</a>`, mLink, esc(name))
		rarity := e.item.Rarity
		if rarity != "" {
			rarity += " "
		}
		if e.item.Amount > 1 {
			fmt.Fprintf(&sb, "%s%s%s%s x%d — %s%.2f\n",
				rarity, badge, linkedName, extras.String(), e.item.Amount, sym, val)
		} else {
			fmt.Fprintf(&sb, "%s%s%s%s — %s%.2f\n",
				rarity, badge, linkedName, extras.String(), sym, e.price)
		}
	}
	if len(priced) > limit {
		fmt.Fprintf(&sb, msgs.InventoryMore, len(priced)-limit)
	}
	fmt.Fprintf(&sb, msgs.InventoryTotal, min(limit, len(priced)), sym, topTotal)
	if noPrice > 0 {
		fmt.Fprintf(&sb, "\n"+msgs.NoPriceWarning, noPrice)
	}

	// Build number buttons for item detail (5 per row).
	invRows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnChanges, "changes:"+steamID+":24h"),
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnValue, "value:"+steamID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.Refresh, "inventory_force:"+steamID),
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnTrack, "track:"+steamID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnEntryPrices, "entries:"+steamID+":0"),
		),
	}
	// Add one numbered button per shown item (5 per row) so user can tap to open detail.
	var numRow []tgbotapi.InlineKeyboardButton
	for i := range topN {
		numRow = append(numRow, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d", i+1),
			fmt.Sprintf("item_detail:%s:%d", steamID, i),
		))
		if len(numRow) == 5 || i == len(topN)-1 {
			invRows = append(invRows, numRow)
			numRow = nil
		}
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(invRows...)
	edit := tgbotapi.NewEditMessageText(chatID, progressMsg.MessageID, sb.String())
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("edit inventory: %v", err)
	}
}

// --- /value ---

func (b *Bot) cmdValue(chatID, userID int64, steamID string) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		accs, _ := b.store.GetAccounts(userID)
		switch len(accs) {
		case 0:
			b.replyHTML(chatID, msgs.AccNone, nil)
			return
		case 1:
			steamID = accs[0].SteamID
		default:
			kb := accSelectKeyboard(accs, "value")
			b.replyHTML(chatID, msgs.SelectAccount, kb)
			return
		}
	}
	if !b.tryLock(userID) {
		b.sendText(chatID, msgs.WorkingBusy)
		return
	}
	defer b.unlock(userID)

	progressMsg, _ := b.api.Send(tgbotapi.NewMessage(chatID, msgs.LoadingInventory))

	items, _, err := b.loadInventory(steamID, false)
	if err != nil {
		b.editText(chatID, progressMsg.MessageID, steamErrText(err, msgs))
		return
	}

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol
	var total float64
	var counted int

	type valuedItem struct {
		marketHashName string
		rarity         string
		unitPrice      float64
		amount         int
	}
	var valuedItems []valuedItem
	rarityTotals := make(map[string]float64)

	// Use DB prices only — no live Steam API calls.
	for _, it := range filterMarketable(items) {
		price, _, _ := b.store.GetLatestPrice(it.MarketHashName)
		if price == 0 {
			continue
		}
		val := price * float64(it.Amount)
		total += val
		counted++
		rarityTotals[it.Rarity] += val
		valuedItems = append(valuedItems, valuedItem{it.MarketHashName, it.Rarity, price, it.Amount})
	}

	// Batch-fetch entry prices for portfolio P&L
	names := make([]string, len(valuedItems))
	for i, p := range valuedItems {
		names[i] = p.marketHashName
	}
	entryMap, _ := b.store.GetEntryPricesMap(userID, names)
	var entryTotal float64
	for _, p := range valuedItems {
		if ep, ok := entryMap[p.marketHashName]; ok && ep > 0 {
			entryTotal += ep * float64(p.amount)
		}
	}

	accName := b.store.GetAccountName(userID, steamID)
	var sb strings.Builder
	fmt.Fprintf(&sb, msgs.ValueHeader, esc(accName))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, msgs.ValueItems, counted)
	sb.WriteString("\n")
	fmt.Fprintf(&sb, msgs.ValueTotal, sym, total)

	// Rarity breakdown
	if len(rarityTotals) > 0 && total > 0 {
		sb.WriteString(msgs.ByRarity)
		type rarityEntry struct {
			emoji string
			val   float64
		}
		var rEntries []rarityEntry
		for emoji, val := range rarityTotals {
			rEntries = append(rEntries, rarityEntry{emoji, val})
		}
		sort.Slice(rEntries, func(i, j int) bool { return rEntries[i].val > rEntries[j].val })
		for _, re := range rEntries {
			label := re.emoji
			if label == "" {
				label = "⬜"
			}
			fmt.Fprintf(&sb, "%s %s%.2f (%.0f%%)\n", label, sym, re.val, re.val/total*100)
		}
	}

	// Portfolio P&L vs entry prices
	if entryTotal > 0 {
		pnl := total - entryTotal
		pnlPct := pnl / entryTotal * 100
		arrow := "📈"
		if pnl < 0 {
			arrow = "📉"
		}
		fmt.Fprintf(&sb, "\n%s <b>P&amp;L:</b> %s%+.2f (%+.1f%%)", arrow, sym, pnl, pnlPct)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnInventory, "inventory:"+steamID),
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnChanges, "changes:"+steamID+":24h"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.ExportBtn, "export:"+steamID),
		),
	)
	edit := tgbotapi.NewEditMessageText(chatID, progressMsg.MessageID, sb.String())
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	b.api.Send(edit)
}

// --- /pnl ---

func (b *Bot) cmdPnl(chatID, userID int64) {
	msgs := i18n.Get(b.getLang(userID))

	records, err := b.store.GetAllEntryPrices(userID)
	if err != nil || len(records) == 0 {
		b.replyHTML(chatID, msgs.PnlNoData, nil)
		return
	}

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol

	type pnlEntry struct {
		name    string
		entry   float64
		current float64
		pct     float64
	}

	var entries []pnlEntry
	for _, r := range records {
		cur, _, lerr := b.store.GetLatestPrice(r.MarketHashName)
		if lerr != nil || cur == 0 {
			continue
		}
		pct := (cur - r.EntryPrice) / r.EntryPrice * 100
		entries = append(entries, pnlEntry{r.MarketHashName, r.EntryPrice, cur, pct})
	}

	if len(entries) == 0 {
		b.replyHTML(chatID, msgs.PnlNoData, nil)
		return
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].pct > entries[j].pct })

	var sb strings.Builder
	sb.WriteString(msgs.PnlHeader + "\n\n")

	sb.WriteString(msgs.PnlGainers + "\n")
	shown := 0
	for _, e := range entries {
		if e.pct <= 0 || shown >= 5 {
			break
		}
		fmt.Fprintf(&sb, "  %s (+%.1f%%) — %s%.2f → %s%.2f\n",
			esc(shortName(e.name)), e.pct, sym, e.entry, sym, e.current)
		shown++
	}
	if shown == 0 {
		sb.WriteString("  —\n")
	}

	sb.WriteString("\n" + msgs.PnlLosers + "\n")
	shown = 0
	for i := len(entries) - 1; i >= 0 && shown < 5; i-- {
		e := entries[i]
		if e.pct >= 0 {
			break
		}
		fmt.Fprintf(&sb, "  %s (%.1f%%) — %s%.2f → %s%.2f\n",
			esc(shortName(e.name)), e.pct, sym, e.entry, sym, e.current)
		shown++
	}
	if shown == 0 {
		sb.WriteString("  —\n")
	}

	b.replyHTML(chatID, sb.String(), nil)
}

// --- Item detail card ---

// inventorySortedItems loads the inventory cache and returns marketable items
// sorted exactly like cmdInventory (by value desc, then name asc).
// priceMap contains the latest DB price for each item.
func (b *Bot) inventorySortedItems(steamID string) ([]steam.Item, map[string]float64, error) {
	data, _, cerr := b.store.GetInventoryCache(steamID)
	if cerr != nil || data == nil {
		return nil, nil, nil
	}
	var items []steam.Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, nil, err
	}
	marketable := filterMarketable(items)

	priceMap := make(map[string]float64, len(marketable))
	for _, it := range marketable {
		p, _, _ := b.store.GetLatestPrice(it.MarketHashName)
		priceMap[it.MarketHashName] = p
	}
	sort.Slice(marketable, func(i, j int) bool {
		vi := priceMap[marketable[i].MarketHashName] * float64(marketable[i].Amount)
		vj := priceMap[marketable[j].MarketHashName] * float64(marketable[j].Amount)
		if vi != vj {
			return vi > vj
		}
		return marketable[i].MarketHashName < marketable[j].MarketHashName
	})
	return marketable, priceMap, nil
}

// cmdItemDetail sends a detail card for the item at global position idx
// (in the value-sorted inventory). Always sends a NEW message so the
// inventory list stays visible above it.
func (b *Bot) cmdItemDetail(chatID, userID int64, steamID string, idx int) {
	msgs := i18n.Get(b.getLang(userID))

	items, priceMap, err := b.inventorySortedItems(steamID)
	if err != nil || items == nil || idx < 0 || idx >= len(items) {
		return
	}
	it := items[idx]

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol
	cur := priceMap[it.MarketHashName]
	ep, _ := b.store.GetEntryPrice(userID, it.MarketHashName)

	rarity := it.Rarity
	if rarity != "" {
		rarity += " "
	}
	badge := itemBadge(it.Name, it.Type, it.Locked)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s%s<b>%s</b>\n", rarity, badge, esc(it.Name))
	fmt.Fprintf(&sb, "<i>%s</i>\n\n", esc(it.Type))

	// Price & P&L.
	if cur > 0 {
		fmt.Fprintf(&sb, "💰 <b>%s%.2f</b>", sym, cur)
		if it.Amount > 1 {
			fmt.Fprintf(&sb, " × %d = %s%.2f", it.Amount, sym, cur*float64(it.Amount))
		}
		sb.WriteString("\n")
	}
	if ep > 0 {
		pct := (cur - ep) / ep * 100
		arrow := "📈"
		if pct < 0 {
			arrow = "📉"
		}
		fmt.Fprintf(&sb, "📌 Entry: <b>%s%.2f</b>", sym, ep)
		if cur > 0 && math.Abs(pct) >= 0.5 {
			fmt.Fprintf(&sb, "  %s%+.1f%%", arrow, pct)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("📌 Entry: <i>not set</i>\n")
	}

	// Applied stickers.
	if len(it.Stickers) > 0 {
		sb.WriteString("\n🧩 <b>Stickers:</b> ")
		sb.WriteString(esc(strings.Join(it.Stickers, ", ")))
		sb.WriteString("\n")
	}
	// Applied charm.
	if it.Charm != "" {
		fmt.Fprintf(&sb, "🎀 <b>Charm:</b> %s\n", esc(it.Charm))
	}
	// Name tag.
	if it.NameTag != "" {
		fmt.Fprintf(&sb, "📛 <b>Name Tag:</b> %s\n", esc(it.NameTag))
	}
	// Trade lock.
	if it.Locked {
		sb.WriteString("\n🔒 <i>Trade locked</i>\n")
	}

	// Buttons.
	callbackEdit := fmt.Sprintf("entry_edit_d:%s:%d", steamID, idx)
	mktURL := marketLink(it.MarketHashName)

	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(msgs.BtnEntryPrices, callbackEdit),
	))
	btnRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonURL("🔗 Steam Market", mktURL),
	}
	if it.InspectLink != "" {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonURL("🔍 Inspect", it.InspectLink))
	}
	rows = append(rows, btnRow)

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = kb
	b.api.Send(msg)
}

// cmdItemDetailEdit renders the same detail card but edits an existing message
// (used to restore the card after entry price is saved).
func (b *Bot) cmdItemDetailEdit(chatID, userID int64, steamID string, idx, editMsgID int) {
	msgs := i18n.Get(b.getLang(userID))

	items, priceMap, err := b.inventorySortedItems(steamID)
	if err != nil || items == nil || idx < 0 || idx >= len(items) {
		return
	}
	it := items[idx]

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol
	cur := priceMap[it.MarketHashName]
	ep, _ := b.store.GetEntryPrice(userID, it.MarketHashName)

	rarity := it.Rarity
	if rarity != "" {
		rarity += " "
	}
	badge := itemBadge(it.Name, it.Type, it.Locked)

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s%s<b>%s</b>\n", rarity, badge, esc(it.Name))
	fmt.Fprintf(&sb, "<i>%s</i>\n\n", esc(it.Type))

	if cur > 0 {
		fmt.Fprintf(&sb, "💰 <b>%s%.2f</b>", sym, cur)
		if it.Amount > 1 {
			fmt.Fprintf(&sb, " × %d = %s%.2f", it.Amount, sym, cur*float64(it.Amount))
		}
		sb.WriteString("\n")
	}
	if ep > 0 {
		pct := (cur - ep) / ep * 100
		arrow := "📈"
		if pct < 0 {
			arrow = "📉"
		}
		fmt.Fprintf(&sb, "📌 Entry: <b>%s%.2f</b>", sym, ep)
		if cur > 0 && math.Abs(pct) >= 0.5 {
			fmt.Fprintf(&sb, "  %s%+.1f%%", arrow, pct)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("📌 Entry: <i>not set</i>\n")
	}
	if len(it.Stickers) > 0 {
		sb.WriteString("\n🧩 <b>Stickers:</b> ")
		sb.WriteString(esc(strings.Join(it.Stickers, ", ")))
		sb.WriteString("\n")
	}
	if it.Charm != "" {
		fmt.Fprintf(&sb, "🎀 <b>Charm:</b> %s\n", esc(it.Charm))
	}
	if it.NameTag != "" {
		fmt.Fprintf(&sb, "📛 <b>Name Tag:</b> %s\n", esc(it.NameTag))
	}
	if it.Locked {
		sb.WriteString("\n🔒 <i>Trade locked</i>\n")
	}

	callbackEdit := fmt.Sprintf("entry_edit_d:%s:%d", steamID, idx)
	mktURL := marketLink(it.MarketHashName)

	var rows [][]tgbotapi.InlineKeyboardButton
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(msgs.BtnEntryPrices, callbackEdit),
	))
	btnRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonURL("🔗 Steam Market", mktURL),
	}
	if it.InspectLink != "" {
		btnRow = append(btnRow, tgbotapi.NewInlineKeyboardButtonURL("🔍 Inspect", it.InspectLink))
	}
	rows = append(rows, btnRow)

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	edit := tgbotapi.NewEditMessageText(chatID, editMsgID, sb.String())
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &kb
	b.api.Send(edit)
}

// startEntryEditFromDetail is like startEntryEdit but triggered from the item
// detail card (editMsgID is the detail card message), so after saving we restore
// the detail card instead of the entries page.
func (b *Bot) startEntryEditFromDetail(chatID, userID int64, msgID int, steamID string, idx int) {
	msgs := i18n.Get(b.getLang(userID))

	items, priceMap, err := b.inventorySortedItems(steamID)
	if err != nil || items == nil || idx < 0 || idx >= len(items) {
		return
	}
	item := items[idx]

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol
	existingEP, _ := b.store.GetEntryPrice(userID, item.MarketHashName)
	curPrice := priceMap[item.MarketHashName]

	var epLine string
	if existingEP > 0 {
		epLine = fmt.Sprintf(msgs.EntryEditCurrentYes, sym, existingEP)
	} else {
		epLine = msgs.EntryEditCurrentNo
	}
	var curLine string
	if curPrice > 0 {
		curLine = fmt.Sprintf(msgs.EntryEditMarket, sym, curPrice)
	} else {
		curLine = msgs.EntryEditMarketNone
	}

	rarity := item.Rarity
	if rarity != "" {
		rarity += " "
	}
	text := fmt.Sprintf(msgs.EntryEditPrompt, rarity, esc(item.Name), epLine, curLine)

	b.states.set(userID, userState{
		id:                  stateAwaitEntryPrice,
		pendingSteamID:      steamID,
		pendingItemName:     item.MarketHashName,
		pendingItemDisplay:  item.Name,
		pendingMsgID:        msgID,
		pendingReturnDetail: true,
		pendingDetailIdx:    idx,
	})

	cancelKb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				msgs.EntryEditCancel,
				fmt.Sprintf("item_detail_restore:%s:%d", steamID, idx),
			),
		),
	)
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &cancelKb
	b.api.Send(edit)
}

// --- /entries — paginated entry price editor ---

const entryPageSize = 8 // items shown per page

// entryPageItems loads the inventory from cache, sorts by value desc (same order
// every time), and returns the full sorted slice plus a price lookup map.
func (b *Bot) entryPageItems(steamID string, userID int64) ([]steam.Item, map[string]float64, error) {
	data, _, cerr := b.store.GetInventoryCache(steamID)
	if cerr != nil || data == nil {
		return nil, nil, nil // caller checks for nil
	}
	var items []steam.Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, nil, err
	}
	marketable := filterMarketable(items)

	// Build price map first so the sort comparator is O(1).
	priceMap := make(map[string]float64, len(marketable))
	for _, it := range marketable {
		p, _, _ := b.store.GetLatestPrice(it.MarketHashName)
		priceMap[it.MarketHashName] = p
	}
	sort.Slice(marketable, func(i, j int) bool {
		vi := priceMap[marketable[i].MarketHashName] * float64(marketable[i].Amount)
		vj := priceMap[marketable[j].MarketHashName] * float64(marketable[j].Amount)
		if vi != vj {
			return vi > vj
		}
		return marketable[i].MarketHashName < marketable[j].MarketHashName
	})
	return marketable, priceMap, nil
}

// cmdEntryPrices renders the paginated entry-price list.
// editMsgID > 0 → edit that message; 0 → send new message.
func (b *Bot) cmdEntryPrices(chatID, userID int64, steamID string, page, editMsgID int) {
	msgs := i18n.Get(b.getLang(userID))

	marketable, priceMap, err := b.entryPageItems(steamID, userID)
	if err != nil {
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
		return
	}
	if marketable == nil {
		if editMsgID > 0 {
			b.editText(chatID, editMsgID, msgs.EntryNoCacheMsg)
		} else {
			b.replyHTML(chatID, msgs.EntryNoCacheMsg, nil)
		}
		return
	}
	if len(marketable) == 0 {
		b.replyHTML(chatID, msgs.InventoryEmpty, nil)
		return
	}

	// Clamp page.
	total := len(marketable)
	totalPages := (total + entryPageSize - 1) / entryPageSize
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * entryPageSize
	end := start + entryPageSize
	if end > total {
		end = total
	}
	pageItems := marketable[start:end]

	// Batch-fetch entry prices for just this page's names.
	pageNames := make([]string, len(pageItems))
	for i, it := range pageItems {
		pageNames[i] = it.MarketHashName
	}
	entryMap, _ := b.store.GetEntryPricesMap(userID, pageNames)

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol

	accName := b.store.GetAccountName(userID, steamID)
	var sb strings.Builder
	fmt.Fprintf(&sb, msgs.EntryPricesHeader, esc(accName), page+1, totalPages, start+1, end, total)

	for i, it := range pageItems {
		num := start + i + 1
		ep := entryMap[it.MarketHashName]
		cur := priceMap[it.MarketHashName]

		rarity := it.Rarity
		if rarity != "" {
			rarity += " "
		}
		name := shortName(it.Name)

		var epStr string
		if ep > 0 {
			epStr = fmt.Sprintf("%s%.2f", sym, ep)
			if cur > 0 {
				pct := (cur - ep) / ep * 100
				if math.Abs(pct) >= 0.5 {
					if pct > 0 {
						epStr += fmt.Sprintf(" 📈+%.0f%%", pct)
					} else {
						epStr += fmt.Sprintf(" 📉%.0f%%", pct)
					}
				}
			}
		} else {
			epStr = "—"
		}

		fmt.Fprintf(&sb, "\n%d. %s<b>%s</b>  <i>%s</i>", num, rarity, esc(name), epStr)
	}

	// Edit buttons (4 per row).
	var rows [][]tgbotapi.InlineKeyboardButton
	var editRow []tgbotapi.InlineKeyboardButton
	for i := range pageItems {
		editRow = append(editRow, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("✏️ %d", start+i+1),
			fmt.Sprintf("entry_edit:%s:%d:%d", steamID, page, i),
		))
		if len(editRow) == 4 || i == len(pageItems)-1 {
			rows = append(rows, editRow)
			editRow = nil
		}
	}

	// Navigation row.
	var navRow []tgbotapi.InlineKeyboardButton
	if page > 0 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(
			"◀", fmt.Sprintf("entries:%s:%d", steamID, page-1),
		))
	}
	navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(
		fmt.Sprintf("📄 %d/%d", page+1, totalPages), "noop",
	))
	if page < totalPages-1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(
			"▶", fmt.Sprintf("entries:%s:%d", steamID, page+1),
		))
	}
	rows = append(rows, navRow)

	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	if editMsgID > 0 {
		edit := tgbotapi.NewEditMessageText(chatID, editMsgID, sb.String())
		edit.ParseMode = tgbotapi.ModeHTML
		edit.ReplyMarkup = &kb
		b.api.Send(edit)
	} else {
		msg := tgbotapi.NewMessage(chatID, sb.String())
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = kb
		b.api.Send(msg)
	}
}

// startEntryEdit switches the message to an entry-price input prompt for the
// item at position idx on the given page.
func (b *Bot) startEntryEdit(chatID, userID int64, msgID int, steamID string, page, idx int) {
	msgs := i18n.Get(b.getLang(userID))

	marketable, priceMap, err := b.entryPageItems(steamID, userID)
	if err != nil || marketable == nil {
		return
	}

	globalIdx := page*entryPageSize + idx
	if globalIdx < 0 || globalIdx >= len(marketable) {
		return
	}
	item := marketable[globalIdx]

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol

	// Existing entry price (if any).
	existingEP, _ := b.store.GetEntryPrice(userID, item.MarketHashName)
	curPrice := priceMap[item.MarketHashName]

	var epLine string
	if existingEP > 0 {
		epLine = fmt.Sprintf(msgs.EntryEditCurrentYes, sym, existingEP)
	} else {
		epLine = msgs.EntryEditCurrentNo
	}
	var curLine string
	if curPrice > 0 {
		curLine = fmt.Sprintf(msgs.EntryEditMarket, sym, curPrice)
	} else {
		curLine = msgs.EntryEditMarketNone
	}

	rarity := item.Rarity
	if rarity != "" {
		rarity += " "
	}
	text := fmt.Sprintf(msgs.EntryEditPrompt, rarity, esc(item.Name), epLine, curLine)

	// Save state so the next text message is treated as the price.
	b.states.set(userID, userState{
		id:                 stateAwaitEntryPrice,
		pendingSteamID:     steamID,
		pendingItemName:    item.MarketHashName,
		pendingItemDisplay: item.Name,
		pendingEntryPage:   page,
		pendingMsgID:       msgID,
	})

	cancelKb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				msgs.EntryEditCancel,
				fmt.Sprintf("entry_cancel:%s:%d", steamID, page),
			),
		),
	)
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &cancelKb
	b.api.Send(edit)
}

// handleEntryPriceInput processes the price the user typed while in stateAwaitEntryPrice.
func (b *Bot) handleEntryPriceInput(chatID, userID int64, text string, st userState, msgs i18n.M) {
	// Accept both dot and comma as decimal separator.
	text = strings.TrimSpace(strings.ReplaceAll(text, ",", "."))
	price, err := strconv.ParseFloat(text, 64)
	if err != nil || price <= 0 {
		b.replyHTML(chatID, msgs.EntryEditInvalid, nil)
		return
	}
	if err := b.store.UpdateEntryPrice(userID, st.pendingItemName, price); err != nil {
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
		return
	}
	b.states.clear(userID)

	currency := b.getUserCurrency(userID)
	sym := currency.Symbol
	b.replyHTML(chatID,
		fmt.Sprintf(msgs.EntryPriceSaved, esc(shortName(st.pendingItemDisplay)), sym, price),
		nil)

	// Restore the originating view in the original message.
	if st.pendingMsgID > 0 {
		if st.pendingReturnDetail {
			// Came from item detail card → re-render the card with updated entry price.
			b.cmdItemDetailEdit(chatID, userID, st.pendingSteamID, st.pendingDetailIdx, st.pendingMsgID)
		} else {
			// Came from entries page → restore the paginated list.
			b.cmdEntryPrices(chatID, userID, st.pendingSteamID, st.pendingEntryPage, st.pendingMsgID)
		}
	}
}

// --- /setentry ---

func (b *Bot) cmdSetEntry(chatID, userID int64, args string) {
	msgs := i18n.Get(b.getLang(userID))
	tokens := strings.Fields(args)
	if len(tokens) < 2 {
		b.sendText(chatID, "Usage: /setentry <item_name> <price>")
		return
	}
	priceStr := tokens[len(tokens)-1]
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		b.sendText(chatID, "Invalid price value")
		return
	}
	itemName := strings.Join(tokens[:len(tokens)-1], " ")
	if err := b.store.UpdateEntryPrice(userID, itemName, price); err != nil {
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
		return
	}
	b.replyHTML(chatID, msgs.EntrySet, nil)
}

// --- /export ---

var rarityLabel = map[string]string{
	"⬜": "Consumer",
	"🟦": "Industrial",
	"🔵": "Mil-Spec",
	"🟣": "Restricted",
	"🔴": "Covert",
	"🟡": "Extraordinary",
	"🔶": "Contraband",
}

func (b *Bot) cmdExport(chatID, userID int64, steamID string) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		accs, _ := b.store.GetAccounts(userID)
		switch len(accs) {
		case 0:
			b.replyHTML(chatID, msgs.AccNone, nil)
			return
		case 1:
			steamID = accs[0].SteamID
		default:
			kb := accSelectKeyboard(accs, "export")
			b.replyHTML(chatID, msgs.SelectAccount, kb)
			return
		}
	}
	if !b.tryLock(userID) {
		b.sendText(chatID, msgs.WorkingBusy)
		return
	}
	defer b.unlock(userID)

	progressMsg, _ := b.api.Send(tgbotapi.NewMessage(chatID, msgs.ExportBuilding))

	items, _, err := b.loadInventory(steamID, false)
	if err != nil {
		b.editText(chatID, progressMsg.MessageID, steamErrText(err, msgs))
		return
	}
	marketable := filterMarketable(items)
	if len(marketable) == 0 {
		b.editText(chatID, progressMsg.MessageID, msgs.ExportNoData)
		return
	}

	// Batch-fetch DB prices and entry prices.
	names := make([]string, len(marketable))
	for i, it := range marketable {
		names[i] = it.MarketHashName
	}
	entryMap, _ := b.store.GetEntryPricesMap(userID, names)

	currency := b.getUserCurrency(userID)
	currCode := b.store.GetCurrency(userID)

	// Build CSV in memory.
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"Name",
		"Amount",
		"Rarity",
		"Current Price (" + currCode + ")",
		"Entry Price (" + currCode + ")",
		"P&L",
		"P&L %",
	})

	for _, it := range marketable {
		cur, _, _ := b.store.GetLatestPrice(it.MarketHashName)
		entry := entryMap[it.MarketHashName]

		curStr, entryStr, pnlStr, pnlPctStr := "", "", "", ""
		if cur > 0 {
			curStr = fmt.Sprintf("%.2f", cur)
		}
		if entry > 0 {
			entryStr = fmt.Sprintf("%.2f", entry)
		}
		if cur > 0 && entry > 0 {
			pnl := cur - entry
			pnlPct := pnl / entry * 100
			pnlStr = fmt.Sprintf("%+.2f", pnl)
			pnlPctStr = fmt.Sprintf("%+.1f%%", pnlPct)
		}

		label, ok := rarityLabel[it.Rarity]
		if !ok {
			label = it.Rarity
		}

		_ = w.Write([]string{
			it.MarketHashName,
			strconv.Itoa(it.Amount),
			label,
			curStr,
			entryStr,
			pnlStr,
			pnlPctStr,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		b.editText(chatID, progressMsg.MessageID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}

	// Delete the progress message.
	b.api.Send(tgbotapi.NewDeleteMessage(chatID, progressMsg.MessageID))

	accName := b.store.GetAccountName(userID, steamID)
	fileName := fmt.Sprintf("inventory_%s_%s.csv", steamID, time.Now().UTC().Format("20060102"))
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
		Name:  fileName,
		Bytes: buf.Bytes(),
	})
	doc.Caption = fmt.Sprintf("📥 <b>%s</b> — %d items  |  %s%s",
		esc(accName), len(marketable), currency.Symbol,
		func() string {
			// Inline total from latest cached prices.
			var t float64
			for _, it := range marketable {
				if cur, _, err := b.store.GetLatestPrice(it.MarketHashName); err == nil && cur > 0 {
					t += cur * float64(it.Amount)
				}
			}
			if t == 0 {
				return "—"
			}
			return fmt.Sprintf("%.2f", t)
		}(),
	)
	doc.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(doc); err != nil {
		log.Printf("send export doc: %v", err)
		b.replyHTML(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())), nil)
	}
}

// --- /profile (account quick-view) ---

func (b *Bot) cmdProfile(chatID, userID int64, steamID string) {
	msgs := i18n.Get(b.getLang(userID))
	accName := b.store.GetAccountName(userID, steamID)
	currency := b.getUserCurrency(userID)
	sym := currency.Symbol

	// Try to get cached inventory to compute portfolio stats.
	data, cachedAt, _ := b.store.GetInventoryCache(steamID)

	var sb strings.Builder
	fmt.Fprintf(&sb, "👤 <b>%s</b>\n<code>%s</code>\n\n",
		esc(accName), steamID)

	if data != nil {
		var items []steam.Item
		if jsonErr := json.Unmarshal(data, &items); jsonErr == nil {
			marketable := filterMarketable(items)
			var total float64
			var priced int
			for _, it := range marketable {
				price, _, _ := b.store.GetLatestPrice(it.MarketHashName)
				if price > 0 {
					total += price * float64(it.Amount)
					priced++
				}
			}
			fmt.Fprintf(&sb, "📦 Items: <b>%d</b> (marketable: %d)\n",
				len(items), len(marketable))
			if total > 0 {
				fmt.Fprintf(&sb, "💰 Portfolio: <b>%s%.2f</b> (%d priced)\n",
					sym, total, priced)
			}
			fmt.Fprintf(&sb, "🕒 Last scan: <b>%s ago</b>\n", formatAge(cachedAt))
		}
	} else {
		sb.WriteString("📦 No cached data — scan inventory first.\n")
	}

	steamURL := "https://steamcommunity.com/profiles/" + steamID
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🌐 Steam Profile", steamURL),
			tgbotapi.NewInlineKeyboardButtonData(msgs.MenuInventory, "inventory:"+steamID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnValue, "value:"+steamID),
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnEntryPrices, "entries:"+steamID+":0"),
		),
	)
	b.replyHTML(chatID, sb.String(), &keyboard)
}

// formatAge returns a human-readable duration since t (e.g. "3m", "2h 15m").
func formatAge(t time.Time) string {
	d := time.Since(t).Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// --- /changes ---

func (b *Bot) cmdChanges(chatID, userID int64, steamID, period string) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		accs, _ := b.store.GetAccounts(userID)
		switch len(accs) {
		case 0:
			b.replyHTML(chatID, msgs.AccNone, nil)
			return
		case 1:
			steamID = accs[0].SteamID
		default:
			kb := accSelectKeyboard(accs, "changes")
			b.replyHTML(chatID, msgs.SelectAccount, kb)
			return
		}
	}
	dur, ok := parsePeriod(period)
	if !ok {
		b.sendText(chatID, "Period: 24h, 7d or 30d")
		return
	}
	if !b.tryLock(userID) {
		b.sendText(chatID, msgs.WorkingBusy)
		return
	}
	defer b.unlock(userID)

	progressMsg, _ := b.api.Send(tgbotapi.NewMessage(chatID, msgs.LoadingInventory))

	items, _, err := b.loadInventory(steamID, false)
	if err != nil {
		b.editText(chatID, progressMsg.MessageID, steamErrText(err, msgs))
		return
	}

	type change struct {
		name     string
		current  float64
		previous float64
		pct      float64
	}

	currency := b.getUserCurrency(userID)
	pastTime := time.Now().Add(-dur)
	var changes []change
	sym := currency.Symbol

	// Use DB prices — no live Steam API calls.
	for _, it := range filterMarketable(items) {
		current, _, _ := b.store.GetLatestPrice(it.MarketHashName)
		if current == 0 {
			continue
		}
		prev, _ := b.store.GetPriceAt(it.MarketHashName, pastTime)
		if prev == 0 {
			continue
		}
		changes = append(changes, change{
			name:     it.Name,
			current:  current,
			previous: prev,
			pct:      (current - prev) / prev * 100,
		})
	}

	if len(changes) == 0 {
		b.editText(chatID, progressMsg.MessageID, fmt.Sprintf(msgs.ChangesNoData, period))
		return
	}

	sort.Slice(changes, func(i, j int) bool {
		return math.Abs(changes[i].pct) > math.Abs(changes[j].pct)
	})

	accName := b.store.GetAccountName(userID, steamID)
	var sb strings.Builder
	fmt.Fprintf(&sb, msgs.ChangesHeader, period, esc(accName))
	for _, c := range changes[:min(15, len(changes))] {
		arrow := "📈"
		if c.pct < 0 {
			arrow = "📉"
		}
		fmt.Fprintf(&sb, "%s %s\n   %s%.2f → %s%.2f (%+.1f%%)\n",
			arrow, esc(shortName(c.name)), sym, c.previous, sym, c.current, c.pct)
	}

	var row []tgbotapi.InlineKeyboardButton
	for _, p := range []string{"24h", "7d", "30d"} {
		label := p
		if p == period {
			label = "• " + p + " •"
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, "changes:"+steamID+":"+p))
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(row)

	edit := tgbotapi.NewEditMessageText(chatID, progressMsg.MessageID, sb.String())
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	b.api.Send(edit)
}

// --- /top ---

func (b *Bot) cmdTop(chatID, userID int64, steamID, mode, period string) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		accs, _ := b.store.GetAccounts(userID)
		switch len(accs) {
		case 0:
			b.replyHTML(chatID, msgs.AccNone, nil)
			return
		case 1:
			steamID = accs[0].SteamID
		default:
			kb := accSelectKeyboard(accs, "top")
			b.replyHTML(chatID, msgs.SelectAccount, kb)
			return
		}
	}
	dur, _ := parsePeriod(period)
	if !b.tryLock(userID) {
		b.sendText(chatID, msgs.WorkingBusy)
		return
	}
	defer b.unlock(userID)

	progressMsg, _ := b.api.Send(tgbotapi.NewMessage(chatID, msgs.LoadingInventory))

	items, _, err := b.loadInventory(steamID, false)
	if err != nil {
		b.editText(chatID, progressMsg.MessageID, steamErrText(err, msgs))
		return
	}

	type topEntry struct {
		name    string
		current float64
		pct     float64
	}

	pastTime := time.Now().Add(-dur)
	currency := b.getUserCurrency(userID)
	var entries []topEntry
	sym := currency.Symbol

	// Use DB prices — no live Steam API calls.
	for _, it := range filterMarketable(items) {
		current, _, _ := b.store.GetLatestPrice(it.MarketHashName)
		if current == 0 {
			continue
		}
		prev, _ := b.store.GetPriceAt(it.MarketHashName, pastTime)
		if prev == 0 {
			continue
		}
		entries = append(entries, topEntry{it.Name, current, (current - prev) / prev * 100})
	}

	sort.Slice(entries, func(i, j int) bool {
		if mode == "gainers" {
			return entries[i].pct > entries[j].pct
		}
		return entries[i].pct < entries[j].pct
	})

	var sb strings.Builder
	if mode == "gainers" {
		fmt.Fprintf(&sb, msgs.TopGainers, period)
	} else {
		fmt.Fprintf(&sb, msgs.TopLosers, period)
	}
	if len(entries) == 0 {
		sb.WriteString(msgs.TopNoData)
	} else {
		for i, e := range entries[:min(10, len(entries))] {
			fmt.Fprintf(&sb, "%d. %s\n   %s%.2f (%+.1f%%)\n",
				i+1, esc(shortName(e.name)), sym, e.current, e.pct) // topEntry fields
		}
	}

	otherMode, otherLabel := "losers", "📉"
	if mode == "losers" {
		otherMode, otherLabel = "gainers", "📈"
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(otherLabel, "top:"+steamID+":"+otherMode+":"+period),
			tgbotapi.NewInlineKeyboardButtonData(msgs.BtnInventory, "inventory:"+steamID),
		),
	)
	edit := tgbotapi.NewEditMessageText(chatID, progressMsg.MessageID, sb.String())
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	b.api.Send(edit)
}

// --- /track / /untrack / /status ---

func (b *Bot) cmdTrack(chatID, userID int64, steamID string) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		b.sendText(chatID, "Usage: /track <steamid64>")
		return
	}
	if err := b.store.TrackInventory(userID, steamID); err != nil {
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	b.replyHTML(chatID, fmt.Sprintf(msgs.TrackAdded, steamID), nil)
}

func (b *Bot) cmdUntrack(chatID, userID int64, steamID string) {
	msgs := i18n.Get(b.getLang(userID))
	if steamID == "" {
		b.sendText(chatID, "Usage: /untrack <steamid64>")
		return
	}
	if err := b.store.UntrackInventory(userID, steamID); err != nil {
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	b.replyHTML(chatID, fmt.Sprintf(msgs.TrackRemoved, steamID), nil)
}

func (b *Bot) cmdStatus(chatID, userID int64) {
	msgs := i18n.Get(b.getLang(userID))
	tracked, err := b.store.GetTrackedInventories(userID)
	if err != nil {
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	if len(tracked) == 0 {
		b.sendText(chatID, msgs.AccNone)
		return
	}
	var sb strings.Builder
	sb.WriteString(msgs.StatusHeader)
	for _, t := range tracked {
		name := b.store.GetAccountName(userID, t.SteamID)
		fmt.Fprintf(&sb, "• <b>%s</b> <code>%s</code> (since %s)\n",
			esc(name), t.SteamID, t.AddedAt.Format("2006-01-02"))
	}
	b.replyHTML(chatID, sb.String(), nil)
}

// --- /alert ---

func (b *Bot) cmdAlert(m *tgbotapi.Message) {
	msgs := i18n.Get(b.getLang(m.From.ID))
	args := strings.Fields(m.CommandArguments())
	if len(args) == 0 {
		b.replyHTML(m.Chat.ID, msgs.HelpText, nil)
		return
	}
	switch args[0] {
	case msgs.AlertList, "list":
		b.alertList(m.Chat.ID, m.From.ID)
	case msgs.AlertRemove, "remove":
		if len(args) < 2 {
			b.sendText(m.Chat.ID, "Usage: /alert remove <id>")
			return
		}
		id, _ := strconv.ParseInt(args[1], 10, 64)
		b.alertRemove(m.Chat.ID, m.From.ID, id)
	case msgs.AlertAdd, "add":
		if len(args) < 3 {
			b.sendText(m.Chat.ID, "Usage: /alert add <item_name> <percent>")
			return
		}
		percentStr := args[len(args)-1]
		itemName := strings.Join(args[1:len(args)-1], " ")
		b.alertAdd(m.Chat.ID, m.From.ID, itemName, percentStr)
	default:
		b.sendText(m.Chat.ID, "Subcommands: add, list, remove")
	}
}

func (b *Bot) alertList(chatID, userID int64) {
	msgs := i18n.Get(b.getLang(userID))
	alerts, err := b.store.GetAlerts(userID)
	if err != nil {
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	if len(alerts) == 0 {
		b.sendText(chatID, msgs.AlertNone)
		return
	}
	currency := b.getUserCurrency(userID)
	sym := currency.Symbol
	var sb strings.Builder
	sb.WriteString(msgs.AlertHeader)
	for _, a := range alerts {
		fmt.Fprintf(&sb, "[%d] %s\n    ±%.0f%%  %s%.2f\n",
			a.ID, esc(a.MarketHashName), a.Threshold, sym, a.BasePrice)
	}
	b.replyHTML(chatID, sb.String(), nil)
}

func (b *Bot) alertRemove(chatID, userID, id int64) {
	msgs := i18n.Get(b.getLang(userID))
	if err := b.store.RemoveAlert(userID, id); err != nil {
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	b.sendText(chatID, fmt.Sprintf(msgs.AlertRemoved, id))
}

func (b *Bot) alertAdd(chatID, userID int64, itemName, percentStr string) {
	msgs := i18n.Get(b.getLang(userID))
	threshold, err := strconv.ParseFloat(percentStr, 64)
	if err != nil || threshold <= 0 {
		b.sendText(chatID, "Invalid percent value")
		return
	}
	currency := b.getUserCurrency(userID)
	price, err := b.steam.FetchPrice(itemName, currency.Code)
	if err != nil || price == 0 {
		b.sendText(chatID, fmt.Sprintf("❌ Could not get price for %q", itemName))
		return
	}
	if err := b.store.AddAlert(userID, itemName, threshold, price); err != nil {
		b.sendText(chatID, fmt.Sprintf(msgs.ErrGeneric, esc(err.Error())))
		return
	}
	sym := currency.Symbol
	b.replyHTML(chatID, fmt.Sprintf(msgs.AlertAdded,
		esc(itemName), sym, price, threshold), nil)
}

// --- Alert / report delivery ---

func (b *Bot) sendAlert(userID int64, message string) {
	msg := tgbotapi.NewMessage(userID, message)
	msg.ParseMode = tgbotapi.ModeHTML
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send alert %d: %v", userID, err)
	}
}

// SendAlert is the public entry point used by tracker and reporter packages.
func (b *Bot) SendAlert(userID int64, message string) { b.sendAlert(userID, message) }

// --- Locking ---

func (b *Bot) tryLock(userID int64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.working[userID] {
		return false
	}
	b.working[userID] = true
	return true
}

func (b *Bot) unlock(userID int64) {
	b.mu.Lock()
	b.working[userID] = false
	b.mu.Unlock()
}

// --- Send helpers ---

func (b *Bot) replyHTML(chatID int64, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	if keyboard != nil {
		msg.ReplyMarkup = keyboard
	}
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("send html: %v", err)
	}
}

func (b *Bot) sendText(chatID int64, text string) {
	if _, err := b.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		log.Printf("send text: %v", err)
	}
}

func (b *Bot) editText(chatID int64, msgID int, text string) {
	edit := tgbotapi.NewEditMessageText(chatID, msgID, text)
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("edit text: %v", err)
	}
}

// --- Item display helpers ---

var qualityShort = map[string]string{
	"Factory New":    "FN",
	"Minimal Wear":   "MW",
	"Field-Tested":   "FT",
	"Well-Worn":      "WW",
	"Battle-Scarred": "BS",
}

var qualityNames = []string{
	"Factory New", "Minimal Wear", "Field-Tested", "Well-Worn", "Battle-Scarred",
}

func itemBadge(name, itemType string, locked bool) string {
	var parts []string
	typeLower := strings.ToLower(itemType)
	switch {
	case strings.Contains(typeLower, "container"):
		parts = append(parts, "📦")
	case strings.Contains(typeLower, "sticker"):
		parts = append(parts, "🧩")
	case strings.Contains(typeLower, "charm"):
		parts = append(parts, "🎀")
	case strings.Contains(typeLower, "graffiti"):
		parts = append(parts, "🎨")
	case strings.Contains(typeLower, "music kit"):
		parts = append(parts, "🎵")
	case strings.Contains(typeLower, "agent"),
		strings.Contains(typeLower, "operator"),
		strings.Contains(typeLower, "soldier"):
		parts = append(parts, "🕵️")
	}
	if strings.HasPrefix(name, "★") {
		parts = append(parts, "⭐")
	}
	if strings.Contains(name, "StatTrak™") {
		parts = append(parts, "ST")
	}
	for _, q := range qualityNames {
		if strings.Contains(name, "("+q+")") {
			parts = append(parts, qualityShort[q])
			break
		}
	}
	if locked {
		parts = append(parts, "🔒")
	}
	if len(parts) == 0 {
		return ""
	}
	return "[" + strings.Join(parts, " ") + "] "
}

func shortName(name string) string {
	for _, q := range qualityNames {
		name = strings.TrimSuffix(name, " ("+q+")")
	}
	name = strings.TrimPrefix(name, "★ ")
	return strings.TrimSpace(name)
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func filterMarketable(items []steam.Item) []steam.Item {
	out := make([]steam.Item, 0, len(items))
	for _, it := range items {
		if it.Marketable {
			out = append(out, it)
		}
	}
	return out
}

func parsePeriod(s string) (time.Duration, bool) {
	switch s {
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	}
	return 0, false
}

func isValidSteamID64(s string) bool {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
