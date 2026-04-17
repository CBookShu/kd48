package dsroute

import (
	"context"
	"testing"
)

func TestRouteLoader_Get_NotFound(t *testing.T) {
	loader := NewRouteLoader(nil, "test/routing")
	_, err := loader.Get(context.Background())
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestNewRouteLoader_DefaultPrefix(t *testing.T) {
	loader := NewRouteLoader(nil, "")
	if loader.prefix != "kd48/routing" {
		t.Errorf("expected default prefix 'kd48/routing', got %q", loader.prefix)
	}
}
