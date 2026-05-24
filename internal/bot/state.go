package bot

import "sync"

type stateID int

const (
	stateNone              stateID = iota
	stateAwaitSteamID              // waiting for SteamID64
	stateAwaitAccName              // waiting for account nickname
	stateAwaitEntryPrice           // waiting for manual entry price value
)

type userState struct {
	id             stateID
	pendingSteamID string
	onboarding     bool // true when this state was entered during onboarding

	// entry price edit context (stateAwaitEntryPrice)
	pendingItemName    string // market_hash_name being edited
	pendingItemDisplay string // full item name for display
	pendingEntryPage   int    // page to restore after saving (entries view)
	pendingMsgID       int    // message ID to edit back after saving
	pendingReturnDetail bool  // true → restore item detail; false → restore entries page
	pendingDetailIdx   int    // global index in sorted inventory (for detail restore)
}

type stateManager struct {
	mu     sync.Mutex
	states map[int64]userState
}

func newStateManager() *stateManager {
	return &stateManager{states: make(map[int64]userState)}
}

func (sm *stateManager) get(userID int64) userState {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.states[userID]
}

func (sm *stateManager) set(userID int64, s userState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[userID] = s
}

func (sm *stateManager) clear(userID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.states, userID)
}
