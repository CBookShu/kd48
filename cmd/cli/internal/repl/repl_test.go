package repl

import (
	"testing"
)

func TestNew(t *testing.T) {
	r := New(func() string { return "test> " })

	if r == nil {
		t.Error("New() should return non-nil REPL")
	}
	if r.line == nil {
		t.Error("New() should initialize liner.State")
	}
	if r.promptFn == nil {
		t.Error("New() should set promptFn")
	}
}

func TestPromptFn(t *testing.T) {
	promptCalled := false
	r := New(func() string {
		promptCalled = true
		return "kd48> "
	})

	_ = r.promptFn()

	if !promptCalled {
		t.Error("promptFn should be called when invoked")
	}
}

func TestSetHistory(t *testing.T) {
	r := New(func() string { return "test> " })

	history := []string{"cmd1", "cmd2", "cmd3"}
	r.SetHistory(history)

	// SetHistory should not panic - it appends to liner
	// We can't easily verify contents, but we verify no error
}

func TestAddHistory(t *testing.T) {
	r := New(func() string { return "test> " })

	// Should not panic
	r.AddHistory("user:login john 123")
	r.AddHistory("checkin:do")
}

func TestClose(t *testing.T) {
	r := New(func() string { return "test> " })

	// Should not panic
	r.Close()

	// Close can be called multiple times safely
	r.Close()
}

// TestPrintWelcomeNotLoggedIn tests the welcome output
// Since PrintWelcome prints to stdout, we verify it doesn't panic
func TestPrintWelcomeNotLoggedIn(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintWelcome(false) panicked: %v", r)
		}
	}()
	PrintWelcome(false)
}

// TestPrintWelcomeLoggedIn tests logged in welcome
func TestPrintWelcomeLoggedIn(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintWelcome(true) panicked: %v", r)
		}
	}()
	PrintWelcome(true)
}

// TestPrintStatusNotLoggedIn tests status when not logged in
func TestPrintStatusNotLoggedIn(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintStatus('') panicked: %v", r)
		}
	}()
	PrintStatus("", false, 0)
}

// TestPrintStatusLoggedIn tests status when logged in
func TestPrintStatusLoggedIn(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PrintStatus('john') panicked: %v", r)
		}
	}()
	// Test various states
	PrintStatus("john", false, 0)
	PrintStatus("john", true, 1)
	PrintStatus("john", true, 5)
}
