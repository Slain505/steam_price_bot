package tracker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- LastRun ---

func TestLastRun_ZeroBeforeTick(t *testing.T) {
	tr := New(nil, nil, nil)
	if got := tr.LastRun(); !got.IsZero() {
		t.Errorf("LastRun before any tick: want zero, got %v", got)
	}
}

func TestLastRun_Concurrent(t *testing.T) {
	tr := New(nil, nil, nil)

	now := time.Now()
	tr.runMu.Lock()
	tr.lastRun = now
	tr.runMu.Unlock()

	// 10 concurrent readers — must all see the same value without races.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if got := tr.LastRun(); !got.Equal(now) {
					t.Errorf("concurrent read mismatch: want %v, got %v", now, got)
				}
			}
		}()
	}
	wg.Wait()
}

// --- SetAlertFunc ---

func TestSetAlertFunc_StoresFunc(t *testing.T) {
	tr := New(nil, nil, nil)
	var called atomic.Bool
	tr.SetAlertFunc(func(userID int64, message string) {
		called.Store(true)
	})
	if tr.onAlert == nil {
		t.Fatal("onAlert should be set")
	}
	tr.onAlert(0, "")
	if !called.Load() {
		t.Error("alert func should have been called")
	}
}

func TestFireTestAlerts_NoOpWithoutAlertFunc(t *testing.T) {
	tr := New(nil, nil, nil)
	// Must not panic even without store/alertFunc.
	tr.FireTestAlerts()
}

// --- ForceRefresh / tickMu ---

func TestForceRefresh_FailsIfTickInProgress(t *testing.T) {
	tr := New(nil, nil, nil)

	// Simulate a tick in progress by locking tickMu ourselves.
	tr.tickMu.Lock()
	defer tr.tickMu.Unlock()

	if ok := tr.ForceRefresh(); ok {
		t.Error("ForceRefresh should return false when tickMu is held")
	}
}

func TestForceRefresh_SucceedsWhenIdle(t *testing.T) {
	tr := New(nil, nil, nil)
	// ForceRefresh spawns a goroutine that calls tick(). tick() will panic
	// without a real store/steam — but the deferred recover() catches it,
	// so the goroutine exits cleanly.
	if ok := tr.ForceRefresh(); !ok {
		t.Error("ForceRefresh on idle tracker should return true")
	}
	// Give the spawned goroutine time to acquire+release the lock.
	time.Sleep(50 * time.Millisecond)
}

// --- Stop ---

func TestStop_ClosesQuitChannel(t *testing.T) {
	tr := New(nil, nil, nil)
	tr.Stop()
	// Reading from a closed channel returns zero value immediately.
	select {
	case <-tr.quit:
		// ok — channel is closed
	case <-time.After(50 * time.Millisecond):
		t.Error("quit channel should be closed after Stop()")
	}
}

// --- Cookie auto-detect ---

func TestNotifyCookieIssue_NoOpWithoutHandler(t *testing.T) {
	tr := New(nil, nil, nil)
	// Should not panic, simply do nothing.
	tr.notifyCookieIssue("76561198000000001", "test detail")
}

func TestNotifyCookieIssue_DispatchesToAdmin(t *testing.T) {
	tr := New(nil, nil, nil)
	var got string
	tr.SetAdminNotifyFunc(func(msg string) { got = msg })

	tr.notifyCookieIssue("76561198000000001", "fixture reason")

	if got == "" {
		t.Fatal("admin notify func should have been called")
	}
	if !contains(got, "76561198000000001") {
		t.Errorf("notification should include steamID, got: %s", got)
	}
	if !contains(got, "fixture reason") {
		t.Errorf("notification should include detail, got: %s", got)
	}
}

func TestNotifyCookieIssue_DedupsWithin24h(t *testing.T) {
	tr := New(nil, nil, nil)
	var count int
	tr.SetAdminNotifyFunc(func(msg string) { count++ })

	tr.notifyCookieIssue("76561198000000001", "first")
	tr.notifyCookieIssue("76561198000000001", "second")
	tr.notifyCookieIssue("76561198000000001", "third")

	if count != 1 {
		t.Errorf("expected 1 notification (deduped), got %d", count)
	}
}

func TestNotifyCookieIssue_DifferentSteamIDsBothFire(t *testing.T) {
	tr := New(nil, nil, nil)
	var count int
	tr.SetAdminNotifyFunc(func(msg string) { count++ })

	tr.notifyCookieIssue("76561198000000001", "user 1")
	tr.notifyCookieIssue("76561198000000002", "user 2")

	if count != 2 {
		t.Errorf("expected 2 notifications (per-steamID dedup), got %d", count)
	}
}

func TestNotifyCookieIssue_FiresAgainAfterCooldown(t *testing.T) {
	tr := New(nil, nil, nil)
	var count int
	tr.SetAdminNotifyFunc(func(msg string) { count++ })

	// First call — fires.
	tr.notifyCookieIssue("76561198000000001", "first")
	// Backdate the last-alert timestamp past the cooldown window.
	tr.cookieAlertMu.Lock()
	tr.lastCookieAlert["76561198000000001"] = time.Now().Add(-2 * cookieAlertCooldown)
	tr.cookieAlertMu.Unlock()
	// Second call — should fire again because the cooldown has elapsed.
	tr.notifyCookieIssue("76561198000000001", "second")

	if count != 2 {
		t.Errorf("expected 2 notifications (cooldown elapsed), got %d", count)
	}
}

// --- Heartbeat ---

func TestSendHeartbeats_NoOpWithoutAlertFunc(t *testing.T) {
	tr := New(nil, nil, nil)
	// onAlert is nil → must short-circuit before touching the (nil) store.
	tr.sendHeartbeats()
}

// contains is a tiny strings.Contains shim to keep the test file's imports tidy.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
