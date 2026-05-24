package i18n

type Lang string

const (
	RU Lang = "ru"
	UA Lang = "ua"
	EN Lang = "en"
	KZ Lang = "kz"
	PL Lang = "pl"
	DE Lang = "de"
)

var All = []Lang{RU, UA, EN, KZ, PL, DE}

type M struct {
	// Onboarding
	Welcome        string
	ChooseLang     string
	LangSet        string
	AddFirstAcc    string
	OnboardDone    string

	// Menu buttons
	MenuInventory string
	MenuAccounts  string
	MenuAnalytics string
	MenuAlerts    string
	MenuSettings  string
	MenuHelp      string

	// Accounts
	AccListHeader   string
	AccAdd          string
	AccAskSteamID   string
	AccAskName      string
	AccAdded        string
	AccRemoved      string
	AccNone         string
	AccAlreadyExists string

	// Inventory
	LoadingInventory string
	GettingPrices    string
	PricesProgress   string // "Getting prices: %d / %d"
	InventoryHeader  string // "Inventory (top %d by value)"
	InventoryMore    string // "...%d more items"
	InventoryTotal   string // "Top-%d total: %s%.2f"
	InventoryEmpty   string

	// Common
	ErrGeneric   string
	ErrRateLimit string
	ErrPrivate   string
	ErrNoCS2     string
	Cancel       string
	Back         string
	Refresh      string
	BtnChanges   string
	BtnValue     string
	BtnTrack     string
	BtnInventory string

	// Value
	ValueHeader string // "Portfolio value"
	ValueItems  string // "Items with price: %d"
	ValueTotal  string // "Total: %s%.2f"

	// Changes
	ChangesHeader  string // "Price changes (%s)"
	ChangesNoData  string

	// Top
	TopGainers string
	TopLosers  string
	TopNoData  string

	// Track
	TrackAdded   string
	TrackRemoved string
	StatusHeader string

	// Currency
	CurrencyHeader  string
	CurrencyCurrent string
	CurrencySet     string

	// Alert
	AlertAdded   string
	AlertRemoved string
	AlertNone    string
	AlertHeader  string
	AlertAdd     string
	AlertList    string
	AlertRemove  string

	// Settings
	SettingsLang     string
	SettingsCurrency string
	SettingsReport   string // "📅 Report"

	// Daily report
	ReportChooseHour string // "Choose report time (UTC):"
	ReportEnabled    string // "✅ Daily report set at %02d:00 UTC"
	ReportDisabled   string // "✅ Daily report disabled"
	ReportHeader     string // "📊 Daily Report"
	ReportPortfolio  string // "💹 Portfolio: %s%.2f"
	ReportPnl        string // "P&L vs entry: %s%+.2f (%+.1f%%)"
	ReportMovers     string // "📈 Biggest movers (24h):"
	ReportNoData     string // "No cached prices found."

	// Extra
	SelectAccount  string // "Select account:"
	ComingSoon     string // "Coming soon"
	WorkingBusy    string // "Already running a request…"
	NoPrices        string // "Could not get prices, try later"
	NoPriceWarning  string // "⚠️ %d item(s) without price"
	GettingPricesNew string // "⏳ Fetching %d new item prices (%d cached)…"
	PricesRetrying  string // "⏳ %d/%d — rate limited, retrying in %ds…"
	HelpText        string // full command reference

	// P&L
	PnlHeader  string // "💹 P&L"
	PnlGainers string // "📈 Gainers:"
	PnlLosers  string // "📉 Losers:"
	PnlNoData  string // "No data yet. Scan inventory first."
	EntrySet   string // "✅ Entry price updated"

	// Analytics
	ByRarity   string // "📊 By rarity:"

	// Export
	ExportBtn      string // "📥 Export CSV"
	ExportBuilding string // "⏳ Preparing CSV…"
	ExportNoData   string // "No items to export."

	// Entry price management
	BtnEntryPrices      string // "✏️ Entry Prices"
	EntryPricesHeader   string // "✏️ Entry Prices — Name | Page X/Y · pos A–B of N"
	EntryNoCacheMsg     string // "No cached inventory. Scan first."
	EntryEditPrompt     string // full prompt for price input
	EntryEditCurrentYes string // "Entry: $15.20"
	EntryEditCurrentNo  string // "Entry: not set"
	EntryEditMarket     string // "Market: $16.80"
	EntryEditMarketNone string // "Market: —"
	EntryEditInvalid    string // "❌ Invalid price"
	EntryEditCancel     string // "❌ Cancel"
	EntryPriceSaved     string // "✅ item — entry: $15.20"
}

var translations = map[Lang]M{
	RU: {
		Welcome:        "👋 Добро пожаловать в <b>CS2 Price Tracker</b>!\n\nОтслеживай цены скинов, получай отчёты и алерты.\n\nВыбери язык:",
		ChooseLang:     "🌍 Выбери язык:",
		LangSet:        "✅ Язык установлен: Русский",
		AddFirstAcc:    "Отлично! Теперь добавь первый Steam-аккаунт для отслеживания.\n\nОтправь <b>SteamID64</b> (17-значное число).\nНайти свой ID: https://steamid.io",
		OnboardDone:    "🎉 Всё готово! Используй меню ниже для навигации.",
		MenuInventory:  "📦 Инвентарь",
		MenuAccounts:   "👥 Аккаунты",
		MenuAnalytics:  "📊 Аналитика",
		MenuAlerts:     "🔔 Алерты",
		MenuSettings:   "⚙️ Настройки",
		MenuHelp:       "❓ Помощь",
		AccListHeader:  "👥 <b>Твои аккаунты</b>",
		AccAdd:         "➕ Добавить аккаунт",
		AccAskSteamID:  "Отправь <b>SteamID64</b> аккаунта (17-значное число).\nНайти: https://steamid.io\n\n/cancel — отмена",
		AccAskName:     "Придумай имя для аккаунта (например «Основной»):\n\n/cancel — отмена",
		AccAdded:       "✅ Аккаунт <b>%s</b> добавлен!\n<code>%s</code>",
		AccRemoved:     "✅ Аккаунт <b>%s</b> удалён",
		AccNone:        "У тебя нет аккаунтов. Нажми «Добавить аккаунт».",
		AccAlreadyExists: "Этот SteamID уже добавлен.",
		LoadingInventory: "⏳ Загружаю инвентарь…",
		GettingPrices:    "⏳ Нашёл %d предметов. Получаю цены…",
		PricesProgress:   "⏳ Получаю цены: %d / %d…",
		InventoryHeader:  "📦 <b>%s</b> — инвентарь (топ %d)\n\n",
		InventoryMore:    "\n<i>…ещё %d предметов</i>\n",
		InventoryTotal:   "\n💰 <b>Топ-%d: %s%.2f</b>",
		InventoryEmpty:   "Инвентарь пуст или нет предметов для продажи.",
		ErrGeneric:       "❌ %s",
		ErrRateLimit:     "⏳ Steam временно ограничил запросы. Попробуй через минуту.",
		ErrPrivate:       "🔒 Инвентарь закрыт. Открой его в настройках Steam.",
		ErrNoCS2:         "❌ Нет предметов CS2 или инвентарь закрыт.",
		Cancel:           "❌ Отменено",
		Back:             "◀️ Назад",
		Refresh:          "🔄 Обновить",
		BtnChanges:       "📊 Изменения 24ч",
		BtnValue:         "💰 Стоимость",
		BtnTrack:         "🔔 Отслеживать",
		BtnInventory:     "📦 Инвентарь",
		ValueHeader:      "💰 <b>Стоимость инвентаря — %s</b>",
		ValueItems:       "Предметов: %d",
		ValueTotal:       "<b>Итого: %s%.2f</b>",
		ChangesHeader:    "📊 <b>Изменения цен (%s) — %s</b>\n\n",
		ChangesNoData:    "Нет данных за %s. Включи /track для накопления истории.",
		TopGainers:       "📈 <b>Топ роста (%s)</b>\n\n",
		TopLosers:        "📉 <b>Топ падения (%s)</b>\n\n",
		TopNoData:        "Нет данных. Включи /track для накопления истории.",
		TrackAdded:       "✅ Отслеживаю <code>%s</code>\nЦены обновляются каждый час.",
		TrackRemoved:     "✅ Остановил отслеживание <code>%s</code>",
		StatusHeader:     "📋 <b>Отслеживаемые аккаунты</b>\n\n",
		CurrencyHeader:   "💱 <b>Валюта</b>",
		CurrencyCurrent:  "Сейчас: <b>%s</b>\n\nВыбери:",
		CurrencySet:      "✅ Валюта: <b>%s %s</b>",
		AlertAdded:       "✅ Алерт добавлен\n%s\nБаза: %s%.2f  Порог: ±%.0f%%",
		AlertRemoved:     "✅ Алерт #%d удалён",
		AlertNone:        "Нет алертов. Добавь: /alert add <предмет> <процент>",
		AlertHeader:      "🔔 <b>Твои алерты</b>\n\n",
		AlertAdd:         "add",
		AlertList:        "list",
		AlertRemove:      "remove",
		SettingsLang:     "🌍 Язык",
		SettingsCurrency: "💱 Валюта",
		SelectAccount:    "Выбери аккаунт:",
		ComingSoon:       "🚧 В разработке",
		WorkingBusy:      "⏳ Уже выполняю запрос, подожди…",
		NoPrices:         "Не удалось получить цены. Попробуй позже.",
		NoPriceWarning:   "⚠️ <i>%d предмет(ов) без цены</i>",
		GettingPricesNew: "⏳ Запрашиваю цены: %d новых (%d из кеша)…",
		PricesRetrying:   "⏳ %d/%d — Steam ограничил, повтор через %dс…",
		PnlHeader:        "💹 <b>P&amp;L (Прибыль / Убыток)</b>",
		PnlGainers:       "📈 <b>Топ роста:</b>",
		PnlLosers:        "📉 <b>Топ падения:</b>",
		PnlNoData:        "Нет данных P&amp;L. Сначала просканируй инвентарь.",
		EntrySet:         "✅ Цена входа обновлена",
		ByRarity:         "\n📊 <b>По редкости:</b>\n",
		SettingsReport:   "📅 Отчёт",
		ReportChooseHour: "Выбери время ежедневного отчёта (UTC):",
		ReportEnabled:    "✅ Ежедневный отчёт в %02d:00 UTC",
		ReportDisabled:   "✅ Ежедневный отчёт отключён",
		ReportHeader:     "📊 <b>Ежедневный отчёт</b>",
		ReportPortfolio:  "💹 Портфель: %s%.2f",
		ReportPnl:        "P&amp;L vs покупка: %s%+.2f (%+.1f%%)",
		ReportMovers:     "📈 Движение цен (24ч):",
		ReportNoData:     "Нет кешированных цен.",
		ExportBtn:        "📥 Экспорт CSV",
		ExportBuilding:   "⏳ Подготавливаю файл…",
		ExportNoData:     "Нет данных для экспорта.",
		BtnEntryPrices:      "✏️ Цены входа",
		EntryPricesHeader:   "✏️ <b>Цены входа — %s</b>\n📄 Стр. %d/%d · поз. %d–%d из %d",
		EntryNoCacheMsg:     "Нет данных инвентаря. Сначала просканируй инвентарь.",
		EntryEditPrompt:     "✏️ <b>Установи цену входа:</b>\n\n%s<b>%s</b>\n\n%s\n%s\n\nОтправь сумму (например <code>15.50</code>):",
		EntryEditCurrentYes: "Цена входа: <b>%s%.2f</b>",
		EntryEditCurrentNo:  "Цена входа: <i>не задана</i>",
		EntryEditMarket:     "Рыночная цена: <b>%s%.2f</b>",
		EntryEditMarketNone: "Рыночная цена: —",
		EntryEditInvalid:    "❌ Неверный формат. Отправь число, например <code>15.50</code>",
		EntryEditCancel:     "❌ Отмена",
		EntryPriceSaved:     "✅ <b>%s</b>\nЦена входа: %s%.2f",
		HelpText: `🎮 <b>CS2 Price Tracker</b>

<b>Меню:</b>
📦 Инвентарь — топ предметов с ценами
👥 Аккаунты — управление Steam аккаунтами
🔔 Алерты — уведомления об изменении цен
⚙️ Настройки — язык и валюта

<b>Команды:</b>
/inventory — инвентарь
/value — суммарная стоимость
/changes [24h|7d|30d] — изменения цен
/top [gainers|losers] — топ движения цен
/track &lt;steamid64&gt; — авто-обновление цен
/status — список отслеживаемых
/currency — выбрать валюту
/alert add &lt;предмет&gt; &lt;%&gt; — алерт
/alert list — список алертов
/cancel — отменить действие`,
	},
	UA: {
		Welcome:        "👋 Ласкаво просимо до <b>CS2 Price Tracker</b>!\n\nСтеж за цінами скінів, отримуй звіти та сповіщення.\n\nОбери мову:",
		ChooseLang:     "🌍 Обери мову:",
		LangSet:        "✅ Мову встановлено: Українська",
		AddFirstAcc:    "Чудово! Тепер додай перший Steam-акаунт.\n\nНадішли <b>SteamID64</b> (17-значне число).\nЗнайти свій ID: https://steamid.io",
		OnboardDone:    "🎉 Все готово! Використовуй меню нижче.",
		MenuInventory:  "📦 Інвентар",
		MenuAccounts:   "👥 Акаунти",
		MenuAnalytics:  "📊 Аналітика",
		MenuAlerts:     "🔔 Алерти",
		MenuSettings:   "⚙️ Налаштування",
		MenuHelp:       "❓ Допомога",
		AccListHeader:  "👥 <b>Твої акаунти</b>",
		AccAdd:         "➕ Додати акаунт",
		AccAskSteamID:  "Надішли <b>SteamID64</b> (17-значне число).\nЗнайти: https://steamid.io\n\n/cancel — скасування",
		AccAskName:     "Придумай ім'я для акаунту (наприклад «Основний»):\n\n/cancel — скасування",
		AccAdded:       "✅ Акаунт <b>%s</b> додано!\n<code>%s</code>",
		AccRemoved:     "✅ Акаунт <b>%s</b> видалено",
		AccNone:        "У тебе немає акаунтів. Натисни «Додати акаунт».",
		AccAlreadyExists: "Цей SteamID вже додано.",
		LoadingInventory: "⏳ Завантажую інвентар…",
		GettingPrices:    "⏳ Знайдено %d предметів. Отримую ціни…",
		PricesProgress:   "⏳ Отримую ціни: %d / %d…",
		InventoryHeader:  "📦 <b>%s</b> — інвентар (топ %d)\n\n",
		InventoryMore:    "\n<i>…ще %d предметів</i>\n",
		InventoryTotal:   "\n💰 <b>Топ-%d: %s%.2f</b>",
		InventoryEmpty:   "Інвентар порожній або немає предметів для продажу.",
		ErrGeneric:       "❌ %s",
		ErrRateLimit:     "⏳ Steam тимчасово обмежив запити. Спробуй за хвилину.",
		ErrPrivate:       "🔒 Інвентар закрито. Відкрий його в налаштуваннях Steam.",
		ErrNoCS2:         "❌ Немає предметів CS2 або інвентар закрито.",
		Cancel:           "❌ Скасовано",
		Back:             "◀️ Назад",
		Refresh:          "🔄 Оновити",
		BtnChanges:       "📊 Зміни 24г",
		BtnValue:         "💰 Вартість",
		BtnTrack:         "🔔 Стежити",
		BtnInventory:     "📦 Інвентар",
		ValueHeader:      "💰 <b>Вартість інвентаря — %s</b>",
		ValueItems:       "Предметів: %d",
		ValueTotal:       "<b>Разом: %s%.2f</b>",
		ChangesHeader:    "📊 <b>Зміни цін (%s) — %s</b>\n\n",
		ChangesNoData:    "Немає даних за %s. Увімкни /track.",
		TopGainers:       "📈 <b>Топ зростання (%s)</b>\n\n",
		TopLosers:        "📉 <b>Топ падіння (%s)</b>\n\n",
		TopNoData:        "Немає даних. Увімкни /track.",
		TrackAdded:       "✅ Стежу за <code>%s</code>",
		TrackRemoved:     "✅ Зупинив стеження <code>%s</code>",
		StatusHeader:     "📋 <b>Відстежувані акаунти</b>\n\n",
		CurrencyHeader:   "💱 <b>Валюта</b>",
		CurrencyCurrent:  "Зараз: <b>%s</b>\n\nОбери:",
		CurrencySet:      "✅ Валюта: <b>%s %s</b>",
		AlertAdded:       "✅ Алерт додано\n%s\nБаза: %s%.2f  Поріг: ±%.0f%%",
		AlertRemoved:     "✅ Алерт #%d видалено",
		AlertNone:        "Немає алертів.",
		AlertHeader:      "🔔 <b>Твої алерти</b>\n\n",
		AlertAdd:         "add",
		AlertList:        "list",
		AlertRemove:      "remove",
		SettingsLang:     "🌍 Мова",
		SettingsCurrency: "💱 Валюта",
		SelectAccount:    "Обери акаунт:",
		ComingSoon:       "🚧 В розробці",
		WorkingBusy:      "⏳ Вже виконую запит, зачекай…",
		NoPrices:         "Не вдалось отримати ціни. Спробуй пізніше.",
		NoPriceWarning:   "⚠️ <i>%d предмет(ів) без ціни</i>",
		GettingPricesNew: "⏳ Отримую ціни: %d нових (%d з кешу)…",
		PricesRetrying:   "⏳ %d/%d — Steam обмежив, повтор через %dс…",
		PnlHeader:        "💹 <b>P&amp;L (Прибуток / Збиток)</b>",
		PnlGainers:       "📈 <b>Топ зростання:</b>",
		PnlLosers:        "📉 <b>Топ падіння:</b>",
		PnlNoData:        "Немає даних P&amp;L. Спочатку просканируй інвентар.",
		EntrySet:         "✅ Ціну входу оновлено",
		ByRarity:         "\n📊 <b>За рідкістю:</b>\n",
		SettingsReport:   "📅 Звіт",
		ReportChooseHour: "Обери час щоденного звіту (UTC):",
		ReportEnabled:    "✅ Щоденний звіт о %02d:00 UTC",
		ReportDisabled:   "✅ Щоденний звіт вимкнено",
		ReportHeader:     "📊 <b>Щоденний звіт</b>",
		ReportPortfolio:  "💹 Портфель: %s%.2f",
		ReportPnl:        "P&amp;L vs купівля: %s%+.2f (%+.1f%%)",
		ReportMovers:     "📈 Рух цін (24г):",
		ReportNoData:     "Немає кешованих цін.",
		ExportBtn:        "📥 Експорт CSV",
		ExportBuilding:   "⏳ Підготовка файлу…",
		ExportNoData:     "Немає даних для експорту.",
		BtnEntryPrices:      "✏️ Ціни входу",
		EntryPricesHeader:   "✏️ <b>Ціни входу — %s</b>\n📄 Стор. %d/%d · поз. %d–%d із %d",
		EntryNoCacheMsg:     "Немає даних інвентаря. Спочатку просканируй інвентар.",
		EntryEditPrompt:     "✏️ <b>Встанови ціну входу:</b>\n\n%s<b>%s</b>\n\n%s\n%s\n\nНадішли суму (наприклад <code>15.50</code>):",
		EntryEditCurrentYes: "Ціна входу: <b>%s%.2f</b>",
		EntryEditCurrentNo:  "Ціна входу: <i>не задана</i>",
		EntryEditMarket:     "Ринкова ціна: <b>%s%.2f</b>",
		EntryEditMarketNone: "Ринкова ціна: —",
		EntryEditInvalid:    "❌ Невірний формат. Надішли число, наприклад <code>15.50</code>",
		EntryEditCancel:     "❌ Скасування",
		EntryPriceSaved:     "✅ <b>%s</b>\nЦіна входу: %s%.2f",
		HelpText: `🎮 <b>CS2 Price Tracker</b>

<b>Меню:</b>
📦 Інвентар — топ предметів з цінами
👥 Акаунти — керування Steam акаунтами
🔔 Алерти — сповіщення про зміну цін
⚙️ Налаштування — мова та валюта

<b>Команди:</b>
/inventory — інвентар
/value — загальна вартість
/changes [24h|7d|30d] — зміни цін
/track &lt;steamid64&gt; — авто-оновлення цін
/status — список відстежуваних
/currency — вибрати валюту
/cancel — скасувати дію`,
	},
	EN: {
		Welcome:        "👋 Welcome to <b>CS2 Price Tracker</b>!\n\nTrack skin prices, get reports and alerts.\n\nChoose your language:",
		ChooseLang:     "🌍 Choose language:",
		LangSet:        "✅ Language set: English",
		AddFirstAcc:    "Great! Now add your first Steam account.\n\nSend your <b>SteamID64</b> (17-digit number).\nFind it at: https://steamid.io",
		OnboardDone:    "🎉 All set! Use the menu below to navigate.",
		MenuInventory:  "📦 Inventory",
		MenuAccounts:   "👥 Accounts",
		MenuAnalytics:  "📊 Analytics",
		MenuAlerts:     "🔔 Alerts",
		MenuSettings:   "⚙️ Settings",
		MenuHelp:       "❓ Help",
		AccListHeader:  "👥 <b>Your accounts</b>",
		AccAdd:         "➕ Add account",
		AccAskSteamID:  "Send the <b>SteamID64</b> (17-digit number).\nFind it at: https://steamid.io\n\n/cancel — cancel",
		AccAskName:     "Give this account a name (e.g. «Main»):\n\n/cancel — cancel",
		AccAdded:       "✅ Account <b>%s</b> added!\n<code>%s</code>",
		AccRemoved:     "✅ Account <b>%s</b> removed",
		AccNone:        "No accounts yet. Tap «Add account».",
		AccAlreadyExists: "This SteamID is already added.",
		LoadingInventory: "⏳ Loading inventory…",
		GettingPrices:    "⏳ Found %d items. Getting prices…",
		PricesProgress:   "⏳ Getting prices: %d / %d…",
		InventoryHeader:  "📦 <b>%s</b> — inventory (top %d)\n\n",
		InventoryMore:    "\n<i>…%d more items</i>\n",
		InventoryTotal:   "\n💰 <b>Top-%d total: %s%.2f</b>",
		InventoryEmpty:   "Inventory is empty or no marketable items.",
		ErrGeneric:       "❌ %s",
		ErrRateLimit:     "⏳ Steam rate limited the request. Try again in a minute.",
		ErrPrivate:       "🔒 Inventory is private. Make it public in Steam settings.",
		ErrNoCS2:         "❌ No CS2 items or inventory is private.",
		Cancel:           "❌ Cancelled",
		Back:             "◀️ Back",
		Refresh:          "🔄 Refresh",
		BtnChanges:       "📊 Changes 24h",
		BtnValue:         "💰 Value",
		BtnTrack:         "🔔 Track",
		BtnInventory:     "📦 Inventory",
		ValueHeader:      "💰 <b>Portfolio value — %s</b>",
		ValueItems:       "Items: %d",
		ValueTotal:       "<b>Total: %s%.2f</b>",
		ChangesHeader:    "📊 <b>Price changes (%s) — %s</b>\n\n",
		ChangesNoData:    "No data for %s. Enable /track to build history.",
		TopGainers:       "📈 <b>Top gainers (%s)</b>\n\n",
		TopLosers:        "📉 <b>Top losers (%s)</b>\n\n",
		TopNoData:        "No data. Enable /track to build history.",
		TrackAdded:       "✅ Tracking <code>%s</code>",
		TrackRemoved:     "✅ Stopped tracking <code>%s</code>",
		StatusHeader:     "📋 <b>Tracked accounts</b>\n\n",
		CurrencyHeader:   "💱 <b>Currency</b>",
		CurrencyCurrent:  "Current: <b>%s</b>\n\nChoose:",
		CurrencySet:      "✅ Currency set: <b>%s %s</b>",
		AlertAdded:       "✅ Alert added\n%s\nBase: %s%.2f  Threshold: ±%.0f%%",
		AlertRemoved:     "✅ Alert #%d removed",
		AlertNone:        "No alerts. Add one: /alert add <item> <percent>",
		AlertHeader:      "🔔 <b>Your alerts</b>\n\n",
		AlertAdd:         "add",
		AlertList:        "list",
		AlertRemove:      "remove",
		SettingsLang:     "🌍 Language",
		SettingsCurrency: "💱 Currency",
		SelectAccount:    "Select account:",
		ComingSoon:       "🚧 Coming soon",
		WorkingBusy:      "⏳ Already running a request, please wait…",
		NoPrices:         "Could not fetch prices. Try again later.",
		NoPriceWarning:   "⚠️ <i>%d item(s) without price</i>",
		GettingPricesNew: "⏳ Fetching prices: %d new items (%d cached)…",
		PricesRetrying:   "⏳ %d/%d — rate limited, retrying in %ds…",
		PnlHeader:        "💹 <b>P&amp;L (Profit / Loss)</b>",
		PnlGainers:       "📈 <b>Top gainers:</b>",
		PnlLosers:        "📉 <b>Top losers:</b>",
		PnlNoData:        "No P&amp;L data yet. Scan your inventory first.",
		EntrySet:         "✅ Entry price updated",
		ByRarity:         "\n📊 <b>By rarity:</b>\n",
		SettingsReport:   "📅 Report",
		ReportChooseHour: "Choose daily report time (UTC):",
		ReportEnabled:    "✅ Daily report at %02d:00 UTC",
		ReportDisabled:   "✅ Daily report disabled",
		ReportHeader:     "📊 <b>Daily Report</b>",
		ReportPortfolio:  "💹 Portfolio: %s%.2f",
		ReportPnl:        "P&amp;L vs entry: %s%+.2f (%+.1f%%)",
		ReportMovers:     "📈 Biggest movers (24h):",
		ReportNoData:     "No cached prices found.",
		ExportBtn:        "📥 Export CSV",
		ExportBuilding:   "⏳ Preparing CSV file…",
		ExportNoData:     "No items to export.",
		BtnEntryPrices:      "✏️ Entry Prices",
		EntryPricesHeader:   "✏️ <b>Entry Prices — %s</b>\n📄 Page %d/%d · items %d–%d of %d",
		EntryNoCacheMsg:     "No cached inventory. Scan your inventory first.",
		EntryEditPrompt:     "✏️ <b>Set entry price for:</b>\n\n%s<b>%s</b>\n\n%s\n%s\n\nSend the price (e.g. <code>15.50</code>):",
		EntryEditCurrentYes: "Entry: <b>%s%.2f</b>",
		EntryEditCurrentNo:  "Entry: <i>not set</i>",
		EntryEditMarket:     "Market: <b>%s%.2f</b>",
		EntryEditMarketNone: "Market: —",
		EntryEditInvalid:    "❌ Invalid price. Send a number, e.g. <code>15.50</code>",
		EntryEditCancel:     "❌ Cancel",
		EntryPriceSaved:     "✅ <b>%s</b>\nEntry price: %s%.2f",
		HelpText: `🎮 <b>CS2 Price Tracker</b>

<b>Menu:</b>
📦 Inventory — top items with prices
👥 Accounts — manage Steam accounts
🔔 Alerts — price change notifications
⚙️ Settings — language and currency

<b>Commands:</b>
/inventory — inventory
/value — total portfolio value
/changes [24h|7d|30d] — price changes
/top [gainers|losers] — top movers
/track &lt;steamid64&gt; — auto price updates
/status — tracked inventories
/currency — set currency
/alert add &lt;item&gt; &lt;%&gt; — add alert
/alert list — list alerts
/cancel — cancel current action`,
	},
	KZ: {
		Welcome:        "👋 <b>CS2 Price Tracker</b> қош келдіңіз!\n\nСкин бағаларын бақылаңыз, есептер мен хабарландырулар алыңыз.\n\nТілді таңдаңыз:",
		ChooseLang:     "🌍 Тілді таңдаңыз:",
		LangSet:        "✅ Тіл орнатылды: Қазақша",
		AddFirstAcc:    "Тамаша! Енді бірінші Steam аккаунтты қосыңыз.\n\n<b>SteamID64</b> жіберіңіз (17 санды).\nТабу: https://steamid.io",
		OnboardDone:    "🎉 Дайын! Төмендегі мәзірді пайдаланыңыз.",
		MenuInventory:  "📦 Инвентарь",
		MenuAccounts:   "👥 Аккаунттар",
		MenuAnalytics:  "📊 Аналитика",
		MenuAlerts:     "🔔 Хабарлар",
		MenuSettings:   "⚙️ Баптаулар",
		MenuHelp:       "❓ Көмек",
		AccListHeader:  "👥 <b>Аккаунттарыңыз</b>",
		AccAdd:         "➕ Аккаунт қосу",
		AccAskSteamID:  "<b>SteamID64</b> жіберіңіз (17 санды).\nТабу: https://steamid.io\n\n/cancel — болдырмау",
		AccAskName:     "Аккаунтқа атау беріңіз (мысалы «Негізгі»):\n\n/cancel — болдырмау",
		AccAdded:       "✅ <b>%s</b> аккаунты қосылды!\n<code>%s</code>",
		AccRemoved:     "✅ <b>%s</b> аккаунты жойылды",
		AccNone:        "Аккаунттар жоқ. «Аккаунт қосу» түймесін басыңыз.",
		AccAlreadyExists: "Бұл SteamID бұрын қосылған.",
		LoadingInventory: "⏳ Инвентарь жүктелуде…",
		GettingPrices:    "⏳ %d зат табылды. Бағалар алынуда…",
		PricesProgress:   "⏳ Бағалар: %d / %d…",
		InventoryHeader:  "📦 <b>%s</b> — инвентарь (үздік %d)\n\n",
		InventoryMore:    "\n<i>…тағы %d зат</i>\n",
		InventoryTotal:   "\n💰 <b>Үздік-%d: %s%.2f</b>",
		InventoryEmpty:   "Инвентарь бос немесе сатылатын заттар жоқ.",
		ErrGeneric:       "❌ %s",
		ErrRateLimit:     "⏳ Steam сұрауларды шектеді. Бір минуттан кейін қайталаңыз.",
		ErrPrivate:       "🔒 Инвентарь жабық.",
		ErrNoCS2:         "❌ CS2 заттары жоқ немесе инвентарь жабық.",
		Cancel:           "❌ Болдырылмады",
		Back:             "◀️ Артқа",
		Refresh:          "🔄 Жаңарту",
		BtnChanges:       "📊 Өзгерістер 24с",
		BtnValue:         "💰 Құны",
		BtnTrack:         "🔔 Бақылау",
		BtnInventory:     "📦 Инвентарь",
		ValueHeader:      "💰 <b>Инвентарь құны — %s</b>",
		ValueItems:       "Заттар: %d",
		ValueTotal:       "<b>Барлығы: %s%.2f</b>",
		ChangesHeader:    "📊 <b>Баға өзгерістері (%s) — %s</b>\n\n",
		ChangesNoData:    "%s үшін деректер жоқ.",
		TopGainers:       "📈 <b>Өсу рейтингі (%s)</b>\n\n",
		TopLosers:        "📉 <b>Төмендеу рейтингі (%s)</b>\n\n",
		TopNoData:        "Деректер жоқ.",
		TrackAdded:       "✅ <code>%s</code> бақылауда",
		TrackRemoved:     "✅ Бақылау тоқтатылды <code>%s</code>",
		StatusHeader:     "📋 <b>Бақыланатын аккаунттар</b>\n\n",
		CurrencyHeader:   "💱 <b>Валюта</b>",
		CurrencyCurrent:  "Қазір: <b>%s</b>\n\nТаңдаңыз:",
		CurrencySet:      "✅ Валюта: <b>%s %s</b>",
		AlertAdded:       "✅ Хабар қосылды\n%s\nБаза: %s%.2f  Шек: ±%.0f%%",
		AlertRemoved:     "✅ Хабар #%d жойылды",
		AlertNone:        "Хабарлар жоқ.",
		AlertHeader:      "🔔 <b>Хабарларыңыз</b>\n\n",
		AlertAdd:         "add",
		AlertList:        "list",
		AlertRemove:      "remove",
		SettingsLang:     "🌍 Тіл",
		SettingsCurrency: "💱 Валюта",
		SelectAccount:    "Аккаунтты таңдаңыз:",
		ComingSoon:       "🚧 Жақында",
		WorkingBusy:      "⏳ Сұрау орындалуда, күтіңіз…",
		NoPrices:         "Бағалар алынбады. Кейінірек қайталаңыз.",
		NoPriceWarning:   "⚠️ <i>%d затта баға жоқ</i>",
		GettingPricesNew: "⏳ Бағалар: %d жаңа (%d кэштен)…",
		PricesRetrying:   "⏳ %d/%d — Steam шектеу, %dс кейін қайталау…",
		PnlHeader:        "💹 <b>P&amp;L (Пайда / Зиян)</b>",
		PnlGainers:       "📈 <b>Өсу рейтингі:</b>",
		PnlLosers:        "📉 <b>Төмендеу рейтингі:</b>",
		PnlNoData:        "P&amp;L деректері жоқ. Инвентарьды скандаңыз.",
		EntrySet:         "✅ Кіру бағасы жаңартылды",
		ByRarity:         "\n📊 <b>Сиректік бойынша:</b>\n",
		SettingsReport:   "📅 Есеп",
		ReportChooseHour: "Күнделікті есеп уақытын таңдаңыз (UTC):",
		ReportEnabled:    "✅ Күнделікті есеп %02d:00 UTC",
		ReportDisabled:   "✅ Күнделікті есеп өшірілді",
		ReportHeader:     "📊 <b>Күнделікті есеп</b>",
		ReportPortfolio:  "💹 Портфель: %s%.2f",
		ReportPnl:        "P&amp;L: %s%+.2f (%+.1f%%)",
		ReportMovers:     "📈 Баға қозғалысы (24с):",
		ReportNoData:     "Кэштелген бағалар жоқ.",
		ExportBtn:        "📥 CSV экспорт",
		ExportBuilding:   "⏳ Файл дайындалуда…",
		ExportNoData:     "Экспорт деректері жоқ.",
		BtnEntryPrices:      "✏️ Кіру бағалары",
		EntryPricesHeader:   "✏️ <b>Кіру бағалары — %s</b>\n📄 Бет %d/%d · поз. %d–%d / %d",
		EntryNoCacheMsg:     "Инвентарь деректері жоқ. Алдымен инвентарьды скандаңыз.",
		EntryEditPrompt:     "✏️ <b>Кіру бағасын орнату:</b>\n\n%s<b>%s</b>\n\n%s\n%s\n\nБағаны жіберіңіз (мысалы <code>15.50</code>):",
		EntryEditCurrentYes: "Кіру бағасы: <b>%s%.2f</b>",
		EntryEditCurrentNo:  "Кіру бағасы: <i>орнатылмаған</i>",
		EntryEditMarket:     "Нарық бағасы: <b>%s%.2f</b>",
		EntryEditMarketNone: "Нарық бағасы: —",
		EntryEditInvalid:    "❌ Қате формат. Мысалы <code>15.50</code> жіберіңіз",
		EntryEditCancel:     "❌ Болдырмау",
		EntryPriceSaved:     "✅ <b>%s</b>\nКіру бағасы: %s%.2f",
		HelpText: `🎮 <b>CS2 Price Tracker</b>

<b>Мәзір:</b>
📦 Инвентарь — бағалары бар заттар
👥 Аккаунттар — Steam аккаунттарын басқару
🔔 Хабарлар — баға өзгерісі туралы хабарлар
⚙️ Баптаулар — тіл және валюта

<b>Командалар:</b>
/inventory — инвентарь
/value — жалпы құны
/changes [24h|7d|30d] — баға өзгерістері
/track &lt;steamid64&gt; — автоматты жаңарту
/status — бақыланатындар
/cancel — болдырмау`,
	},
	PL: {
		Welcome:        "👋 Witaj w <b>CS2 Price Tracker</b>!\n\nŚledź ceny skinów, otrzymuj raporty i alerty.\n\nWybierz język:",
		ChooseLang:     "🌍 Wybierz język:",
		LangSet:        "✅ Język ustawiony: Polski",
		AddFirstAcc:    "Świetnie! Teraz dodaj pierwsze konto Steam.\n\nWyślij <b>SteamID64</b> (17-cyfrowy numer).\nZnajdź swoje ID: https://steamid.io",
		OnboardDone:    "🎉 Gotowe! Użyj menu poniżej.",
		MenuInventory:  "📦 Ekwipunek",
		MenuAccounts:   "👥 Konta",
		MenuAnalytics:  "📊 Analityka",
		MenuAlerts:     "🔔 Alerty",
		MenuSettings:   "⚙️ Ustawienia",
		MenuHelp:       "❓ Pomoc",
		AccListHeader:  "👥 <b>Twoje konta</b>",
		AccAdd:         "➕ Dodaj konto",
		AccAskSteamID:  "Wyślij <b>SteamID64</b> (17-cyfrowy numer).\nZnajdź: https://steamid.io\n\n/cancel — anuluj",
		AccAskName:     "Podaj nazwę dla tego konta (np. «Główne»):\n\n/cancel — anuluj",
		AccAdded:       "✅ Konto <b>%s</b> dodane!\n<code>%s</code>",
		AccRemoved:     "✅ Konto <b>%s</b> usunięte",
		AccNone:        "Brak kont. Naciśnij «Dodaj konto».",
		AccAlreadyExists: "Ten SteamID już istnieje.",
		LoadingInventory: "⏳ Ładuję ekwipunek…",
		GettingPrices:    "⏳ Znaleziono %d przedmiotów. Pobieranie cen…",
		PricesProgress:   "⏳ Pobieranie cen: %d / %d…",
		InventoryHeader:  "📦 <b>%s</b> — ekwipunek (top %d)\n\n",
		InventoryMore:    "\n<i>…jeszcze %d przedmiotów</i>\n",
		InventoryTotal:   "\n💰 <b>Top-%d łącznie: %s%.2f</b>",
		InventoryEmpty:   "Ekwipunek jest pusty lub brak przedmiotów do sprzedaży.",
		ErrGeneric:       "❌ %s",
		ErrRateLimit:     "⏳ Steam ograniczył zapytania. Spróbuj za chwilę.",
		ErrPrivate:       "🔒 Ekwipunek jest prywatny.",
		ErrNoCS2:         "❌ Brak przedmiotów CS2 lub ekwipunek prywatny.",
		Cancel:           "❌ Anulowano",
		Back:             "◀️ Wróć",
		Refresh:          "🔄 Odśwież",
		BtnChanges:       "📊 Zmiany 24h",
		BtnValue:         "💰 Wartość",
		BtnTrack:         "🔔 Śledź",
		BtnInventory:     "📦 Ekwipunek",
		ValueHeader:      "💰 <b>Wartość ekwipunku — %s</b>",
		ValueItems:       "Przedmiotów: %d",
		ValueTotal:       "<b>Łącznie: %s%.2f</b>",
		ChangesHeader:    "📊 <b>Zmiany cen (%s) — %s</b>\n\n",
		ChangesNoData:    "Brak danych za %s. Włącz /track.",
		TopGainers:       "📈 <b>Największe wzrosty (%s)</b>\n\n",
		TopLosers:        "📉 <b>Największe spadki (%s)</b>\n\n",
		TopNoData:        "Brak danych. Włącz /track.",
		TrackAdded:       "✅ Śledzę <code>%s</code>",
		TrackRemoved:     "✅ Zatrzymano śledzenie <code>%s</code>",
		StatusHeader:     "📋 <b>Śledzone konta</b>\n\n",
		CurrencyHeader:   "💱 <b>Waluta</b>",
		CurrencyCurrent:  "Teraz: <b>%s</b>\n\nWybierz:",
		CurrencySet:      "✅ Waluta: <b>%s %s</b>",
		AlertAdded:       "✅ Alert dodany\n%s\nBaza: %s%.2f  Próg: ±%.0f%%",
		AlertRemoved:     "✅ Alert #%d usunięty",
		AlertNone:        "Brak alertów.",
		AlertHeader:      "🔔 <b>Twoje alerty</b>\n\n",
		AlertAdd:         "add",
		AlertList:        "list",
		AlertRemove:      "remove",
		SettingsLang:     "🌍 Język",
		SettingsCurrency: "💱 Waluta",
		SelectAccount:    "Wybierz konto:",
		ComingSoon:       "🚧 Wkrótce",
		WorkingBusy:      "⏳ Żądanie w toku, poczekaj…",
		NoPrices:         "Nie udało się pobrać cen. Spróbuj później.",
		NoPriceWarning:   "⚠️ <i>%d przedmiot(ów) bez ceny</i>",
		GettingPricesNew: "⏳ Pobieranie cen: %d nowych (%d z cache)…",
		PricesRetrying:   "⏳ %d/%d — limit Steam, ponawianie za %ds…",
		PnlHeader:        "💹 <b>P&amp;L (Zysk / Strata)</b>",
		PnlGainers:       "📈 <b>Największe wzrosty:</b>",
		PnlLosers:        "📉 <b>Największe spadki:</b>",
		PnlNoData:        "Brak danych P&amp;L. Najpierw zeskanuj ekwipunek.",
		EntrySet:         "✅ Cena wejścia zaktualizowana",
		ByRarity:         "\n📊 <b>Według rzadkości:</b>\n",
		SettingsReport:   "📅 Raport",
		ReportChooseHour: "Wybierz godzinę raportu dziennego (UTC):",
		ReportEnabled:    "✅ Raport dzienny o %02d:00 UTC",
		ReportDisabled:   "✅ Raport dzienny wyłączony",
		ReportHeader:     "📊 <b>Raport dzienny</b>",
		ReportPortfolio:  "💹 Portfel: %s%.2f",
		ReportPnl:        "P&amp;L vs kupno: %s%+.2f (%+.1f%%)",
		ReportMovers:     "📈 Największe ruchy (24h):",
		ReportNoData:     "Brak cen w pamięci podręcznej.",
		ExportBtn:        "📥 Eksport CSV",
		ExportBuilding:   "⏳ Przygotowuję plik…",
		ExportNoData:     "Brak danych do eksportu.",
		BtnEntryPrices:      "✏️ Ceny zakupu",
		EntryPricesHeader:   "✏️ <b>Ceny zakupu — %s</b>\n📄 Str. %d/%d · poz. %d–%d z %d",
		EntryNoCacheMsg:     "Brak danych ekwipunku. Najpierw zeskanuj ekwipunek.",
		EntryEditPrompt:     "✏️ <b>Ustaw cenę zakupu dla:</b>\n\n%s<b>%s</b>\n\n%s\n%s\n\nWyślij kwotę (np. <code>15.50</code>):",
		EntryEditCurrentYes: "Cena zakupu: <b>%s%.2f</b>",
		EntryEditCurrentNo:  "Cena zakupu: <i>nie ustawiona</i>",
		EntryEditMarket:     "Cena rynkowa: <b>%s%.2f</b>",
		EntryEditMarketNone: "Cena rynkowa: —",
		EntryEditInvalid:    "❌ Nieprawidłowa cena. Wyślij liczbę, np. <code>15.50</code>",
		EntryEditCancel:     "❌ Anuluj",
		EntryPriceSaved:     "✅ <b>%s</b>\nCena zakupu: %s%.2f",
		HelpText: `🎮 <b>CS2 Price Tracker</b>

<b>Menu:</b>
📦 Ekwipunek — przedmioty z cenami
👥 Konta — zarządzanie kontami Steam
🔔 Alerty — powiadomienia o zmianach cen
⚙️ Ustawienia — język i waluta

<b>Komendy:</b>
/inventory — ekwipunek
/value — łączna wartość
/changes [24h|7d|30d] — zmiany cen
/track &lt;steamid64&gt; — automatyczne aktualizacje
/status — śledzone konta
/cancel — anuluj działanie`,
	},
	DE: {
		Welcome:        "👋 Willkommen bei <b>CS2 Price Tracker</b>!\n\nVerfolge Skin-Preise, erhalte Berichte und Benachrichtigungen.\n\nWähle deine Sprache:",
		ChooseLang:     "🌍 Sprache wählen:",
		LangSet:        "✅ Sprache eingestellt: Deutsch",
		AddFirstAcc:    "Super! Füge jetzt dein erstes Steam-Konto hinzu.\n\nSende deine <b>SteamID64</b> (17-stellige Zahl).\nFinden: https://steamid.io",
		OnboardDone:    "🎉 Fertig! Benutze das Menü unten.",
		MenuInventory:  "📦 Inventar",
		MenuAccounts:   "👥 Konten",
		MenuAnalytics:  "📊 Analytik",
		MenuAlerts:     "🔔 Benachricht.",
		MenuSettings:   "⚙️ Einstellungen",
		MenuHelp:       "❓ Hilfe",
		AccListHeader:  "👥 <b>Deine Konten</b>",
		AccAdd:         "➕ Konto hinzufügen",
		AccAskSteamID:  "Sende die <b>SteamID64</b> (17-stellige Zahl).\nFinden: https://steamid.io\n\n/cancel — abbrechen",
		AccAskName:     "Gib dem Konto einen Namen (z.B. «Haupt»):\n\n/cancel — abbrechen",
		AccAdded:       "✅ Konto <b>%s</b> hinzugefügt!\n<code>%s</code>",
		AccRemoved:     "✅ Konto <b>%s</b> entfernt",
		AccNone:        "Keine Konten. Tippe «Konto hinzufügen».",
		AccAlreadyExists: "Diese SteamID ist bereits vorhanden.",
		LoadingInventory: "⏳ Lade Inventar…",
		GettingPrices:    "⏳ %d Gegenstände gefunden. Preise werden abgerufen…",
		PricesProgress:   "⏳ Preise abrufen: %d / %d…",
		InventoryHeader:  "📦 <b>%s</b> — Inventar (Top %d)\n\n",
		InventoryMore:    "\n<i>…%d weitere Gegenstände</i>\n",
		InventoryTotal:   "\n💰 <b>Top-%d gesamt: %s%.2f</b>",
		InventoryEmpty:   "Inventar ist leer oder keine verkäuflichen Gegenstände.",
		ErrGeneric:       "❌ %s",
		ErrRateLimit:     "⏳ Steam hat Anfragen begrenzt. Versuche es in einer Minute.",
		ErrPrivate:       "🔒 Inventar ist privat.",
		ErrNoCS2:         "❌ Keine CS2-Gegenstände oder Inventar privat.",
		Cancel:           "❌ Abgebrochen",
		Back:             "◀️ Zurück",
		Refresh:          "🔄 Aktualisieren",
		BtnChanges:       "📊 Änderungen 24h",
		BtnValue:         "💰 Wert",
		BtnTrack:         "🔔 Verfolgen",
		BtnInventory:     "📦 Inventar",
		ValueHeader:      "💰 <b>Inventarwert — %s</b>",
		ValueItems:       "Gegenstände: %d",
		ValueTotal:       "<b>Gesamt: %s%.2f</b>",
		ChangesHeader:    "📊 <b>Preisänderungen (%s) — %s</b>\n\n",
		ChangesNoData:    "Keine Daten für %s. Aktiviere /track.",
		TopGainers:       "📈 <b>Größte Gewinner (%s)</b>\n\n",
		TopLosers:        "📉 <b>Größte Verlierer (%s)</b>\n\n",
		TopNoData:        "Keine Daten. Aktiviere /track.",
		TrackAdded:       "✅ Verfolge <code>%s</code>",
		TrackRemoved:     "✅ Verfolgung gestoppt <code>%s</code>",
		StatusHeader:     "📋 <b>Verfolgte Konten</b>\n\n",
		CurrencyHeader:   "💱 <b>Währung</b>",
		CurrencyCurrent:  "Aktuell: <b>%s</b>\n\nWählen:",
		CurrencySet:      "✅ Währung: <b>%s %s</b>",
		AlertAdded:       "✅ Benachrichtigung hinzugefügt\n%s\nBasis: %s%.2f  Schwelle: ±%.0f%%",
		AlertRemoved:     "✅ Benachrichtigung #%d entfernt",
		AlertNone:        "Keine Benachrichtigungen.",
		AlertHeader:      "🔔 <b>Deine Benachrichtigungen</b>\n\n",
		AlertAdd:         "add",
		AlertList:        "list",
		AlertRemove:      "remove",
		SettingsLang:     "🌍 Sprache",
		SettingsCurrency: "💱 Währung",
		SelectAccount:    "Konto auswählen:",
		ComingSoon:       "🚧 Demnächst",
		WorkingBusy:      "⏳ Anfrage läuft bereits, bitte warten…",
		NoPrices:         "Preise konnten nicht abgerufen werden. Später versuchen.",
		NoPriceWarning:   "⚠️ <i>%d Gegenstand/Gegenstände ohne Preis</i>",
		GettingPricesNew: "⏳ Preise abrufen: %d neue (%d aus Cache)…",
		PricesRetrying:   "⏳ %d/%d — Steam-Limit, Wiederholung in %ds…",
		PnlHeader:        "💹 <b>P&amp;L (Gewinn / Verlust)</b>",
		PnlGainers:       "📈 <b>Größte Gewinner:</b>",
		PnlLosers:        "📉 <b>Größte Verlierer:</b>",
		PnlNoData:        "Keine P&amp;L-Daten. Bitte zuerst Inventar scannen.",
		EntrySet:         "✅ Einstiegspreis aktualisiert",
		ByRarity:         "\n📊 <b>Nach Seltenheit:</b>\n",
		SettingsReport:   "📅 Bericht",
		ReportChooseHour: "Wähle die Zeit für den täglichen Bericht (UTC):",
		ReportEnabled:    "✅ Täglicher Bericht um %02d:00 UTC",
		ReportDisabled:   "✅ Täglicher Bericht deaktiviert",
		ReportHeader:     "📊 <b>Täglicher Bericht</b>",
		ReportPortfolio:  "💹 Portfolio: %s%.2f",
		ReportPnl:        "P&amp;L vs Kauf: %s%+.2f (%+.1f%%)",
		ReportMovers:     "📈 Größte Bewegungen (24h):",
		ReportNoData:     "Keine gecachten Preise gefunden.",
		ExportBtn:        "📥 CSV Export",
		ExportBuilding:   "⏳ Datei wird erstellt…",
		ExportNoData:     "Keine Daten zum Exportieren.",
		BtnEntryPrices:      "✏️ Einstiegspreise",
		EntryPricesHeader:   "✏️ <b>Einstiegspreise — %s</b>\n📄 Seite %d/%d · Pos. %d–%d von %d",
		EntryNoCacheMsg:     "Keine Inventardaten. Zuerst Inventar scannen.",
		EntryEditPrompt:     "✏️ <b>Einstiegspreis setzen für:</b>\n\n%s<b>%s</b>\n\n%s\n%s\n\nPreis senden (z.B. <code>15.50</code>):",
		EntryEditCurrentYes: "Einstiegspreis: <b>%s%.2f</b>",
		EntryEditCurrentNo:  "Einstiegspreis: <i>nicht gesetzt</i>",
		EntryEditMarket:     "Marktpreis: <b>%s%.2f</b>",
		EntryEditMarketNone: "Marktpreis: —",
		EntryEditInvalid:    "❌ Ungültiger Preis. Zahl senden, z.B. <code>15.50</code>",
		EntryEditCancel:     "❌ Abbrechen",
		EntryPriceSaved:     "✅ <b>%s</b>\nEinstiegspreis: %s%.2f",
		HelpText: `🎮 <b>CS2 Price Tracker</b>

<b>Menü:</b>
📦 Inventar — Gegenstände mit Preisen
👥 Konten — Steam-Konten verwalten
🔔 Benachricht. — Preisänderungs-Alarme
⚙️ Einstellungen — Sprache und Währung

<b>Befehle:</b>
/inventory — Inventar
/value — Gesamtwert
/changes [24h|7d|30d] — Preisänderungen
/track &lt;steamid64&gt; — automatische Updates
/status — verfolgte Konten
/cancel — Aktion abbrechen`,
	},
}

// Get returns translations for the given language, falling back to English.
func Get(lang Lang) M {
	if m, ok := translations[lang]; ok {
		return m
	}
	return translations[EN]
}

// FromString converts a string to Lang, defaulting to English.
func FromString(s string) Lang {
	l := Lang(s)
	if _, ok := translations[l]; ok {
		return l
	}
	return EN
}

// LangButton returns the display label for a language selection button.
func LangButton(l Lang) string {
	flags := map[Lang]string{
		RU: "🇷🇺 Русский",
		UA: "🇺🇦 Українська",
		EN: "🇬🇧 English",
		KZ: "🇰🇿 Қазақша",
		PL: "🇵🇱 Polski",
		DE: "🇩🇪 Deutsch",
	}
	if s, ok := flags[l]; ok {
		return s
	}
	return string(l)
}
