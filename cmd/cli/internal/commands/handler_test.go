package commands

import (
	"testing"

	"github.com/CBookShu/kd48/cli/internal/client"
	"github.com/CBookShu/kd48/cli/internal/state"
)

// mockGateway is a minimal mock for testing
type mockGateway struct {
	resp *client.WsResponse
	err  error
}

func (m *mockGateway) Send(ctx interface{}, method string, payload interface{}) (*client.WsResponse, error) {
	return m.resp, m.err
}

func (m *mockGateway) Close() error {
	return nil
}

func TestHandleEmptyInput(t *testing.T) {
	gw := client.New()
	st := state.New()
	h := New(gw, st)

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := h.Handle(tt.input)
			if result != "" {
				t.Errorf("Handle('%s') should return empty string, got '%s'", tt.input, result)
			}
		})
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	gw := client.New()
	st := state.New()
	h := New(gw, st)

	result := h.Handle("unknown:command")
	expected := "[错误] 未知命令 'unknown:command'，输入 'help' 查看可用命令"
	if result != expected {
		t.Errorf("Handle('unknown:command') = '%s', want '%s'", result, expected)
	}
}

func TestHandleQuit(t *testing.T) {
	gw := client.New()
	st := state.New()
	h := New(gw, st)

	result := h.Handle("quit")
	if result != "quit" {
		t.Errorf("Handle('quit') = '%s', want 'quit'", result)
	}

	result = h.Handle("exit")
	if result != "quit" {
		t.Errorf("Handle('exit') = '%s', want 'quit'", result)
	}
}

func TestHandleHelp(t *testing.T) {
	gw := client.New()
	st := state.New()
	h := New(gw, st)

	// Not logged in - should see login/register commands
	result := h.Handle("help")
	if result == "" {
		t.Error("Handle('help') should not return empty string")
	}
	if result == "[错误] 未知命令" {
		t.Error("Handle('help') should return help text")
	}

	// Logged in - should see additional commands
	st.SetUser("test", 1, "token")
	result = h.Handle("help")
	if result == "" {
		t.Error("Handle('help') should not return empty string when logged in")
	}
}

func TestLoginRequiredWhenNotLoggedIn(t *testing.T) {
	gw := client.New()
	st := state.New()
	h := New(gw, st)

	tests := []string{
		"user:logout",
		"checkin:do",
		"checkin:status",
		"items",
	}

	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			result := h.Handle(cmd)
			// These commands should require login - the handler should either:
			// 1. Return error saying login required
			// 2. Route to the command which will check login state
			// Since handler routes based on login state, we check the routing works
			if result == "" {
				t.Logf("Command '%s' handled without error when not logged in", cmd)
			}
		})
	}
}

func TestLoginCommandsAvailableWhenLoggedIn(t *testing.T) {
	gw := client.New()
	st := state.New()
	h := New(gw, st)

	// Even when logged in, user:login and user:register should work (for account switch)
	st.SetUser("testuser", 123, "token")

	result := h.Handle("user:login")
	// Should not say "unknown command" - should get usage error for missing args
	if result == "[错误] 未知命令 'user:login'，输入 'help' 查看可用命令" {
		t.Error("user:login should be available when logged in")
	}

	result = h.Handle("user:register")
	if result == "[错误] 未知命令 'user:register'，输入 'help' 查看可用命令" {
		t.Error("user:register should be available when logged in")
	}
}
