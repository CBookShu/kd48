package state

import (
	"testing"
)

func TestNew(t *testing.T) {
	s := New()

	if s.IsLoggedIn {
		t.Error("New() should create state with IsLoggedIn=false")
	}
	if s.Username != "" {
		t.Error("New() should create state with empty Username")
	}
	if s.UserID != 0 {
		t.Error("New() should create state with UserID=0")
	}
	if s.Token != "" {
		t.Error("New() should create state with empty Token")
	}
	if s.TodayChecked {
		t.Error("New() should create state with TodayChecked=false")
	}
	if s.ContinuousDays != 0 {
		t.Error("New() should create state with ContinuousDays=0")
	}
}

func TestReset(t *testing.T) {
	s := New()
	s.IsLoggedIn = true
	s.Username = "testuser"
	s.UserID = 123
	s.Token = "token123"
	s.TodayChecked = true
	s.ContinuousDays = 5

	s.Reset()

	if s.IsLoggedIn {
		t.Error("Reset() should set IsLoggedIn=false")
	}
	if s.Username != "" {
		t.Error("Reset() should clear Username")
	}
	if s.UserID != 0 {
		t.Error("Reset() should clear UserID")
	}
	if s.Token != "" {
		t.Error("Reset() should clear Token")
	}
	if s.TodayChecked {
		t.Error("Reset() should set TodayChecked=false")
	}
	if s.ContinuousDays != 0 {
		t.Error("Reset() should set ContinuousDays=0")
	}
}

func TestSetUser(t *testing.T) {
	s := New()

	s.SetUser("john", 456, "token_abc")

	if !s.IsLoggedIn {
		t.Error("SetUser() should set IsLoggedIn=true")
	}
	if s.Username != "john" {
		t.Errorf("SetUser() should set Username='john', got '%s'", s.Username)
	}
	if s.UserID != 456 {
		t.Errorf("SetUser() should set UserID=456, got %d", s.UserID)
	}
	if s.Token != "token_abc" {
		t.Errorf("SetUser() should set Token='token_abc', got '%s'", s.Token)
	}
}
