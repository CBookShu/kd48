package dsroute

import (
	"errors"
	"testing"
)

func TestResolvePoolName(t *testing.T) {
	tests := []struct {
		name          string
		rules         []RouteRule
		routingKey    string
		wantPool      string
		wantPrefix    string
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name: "longer_prefix_wins",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
				{Prefix: "user:premium:", Pool: "premium-pool"},
				{Prefix: "", Pool: "default-pool"},
			},
			routingKey: "user:premium:123",
			wantPool:   "premium-pool",
			wantPrefix: "user:premium:",
		},
		{
			name: "exact_prefix_match",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
				{Prefix: "", Pool: "default-pool"},
			},
			routingKey: "user:abc",
			wantPool:   "user-pool",
			wantPrefix: "user:",
		},
		{
			name: "fallback_to_empty_prefix",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
				{Prefix: "", Pool: "default-pool"},
			},
			routingKey: "order:123",
			wantPool:   "default-pool",
			wantPrefix: "",
		},
		{
			name: "empty_prefix_only",
			rules: []RouteRule{
				{Prefix: "", Pool: "default-pool"},
			},
			routingKey: "any:key",
			wantPool:   "default-pool",
			wantPrefix: "",
		},
		{
			name:          "no_candidates_no_empty_prefix",
			rules:         []RouteRule{},
			routingKey:    "user:123",
			wantErr:       true,
			wantErrSubstr: "no matching route",
		},
		{
			name: "non_empty_prefixes_no_match_no_empty_prefix",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
				{Prefix: "order:", Pool: "order-pool"},
			},
			routingKey:    "product:123",
			wantErr:       true,
			wantErrSubstr: "no matching route",
		},
		{
			name: "multiple_same_prefix_length_pick_first",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool-a"},
				{Prefix: "user:", Pool: "user-pool-b"},
			},
			routingKey: "user:123",
			wantPool:   "user-pool-a",
			wantPrefix: "user:",
		},
		{
			name: "partial_prefix_not_match",
			rules: []RouteRule{
				{Prefix: "user:premium:", Pool: "premium-pool"},
			},
			routingKey: "user:normal:123",
			wantErr:    true,
		},
		{
			name: "empty_routing_key_with_empty_prefix",
			rules: []RouteRule{
				{Prefix: "", Pool: "default-pool"},
			},
			routingKey: "",
			wantPool:   "default-pool",
			wantPrefix: "",
		},
		{
			name: "empty_routing_key_no_match",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
			},
			routingKey: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPool, gotPrefix, err := ResolvePoolName(tt.rules, tt.routingKey)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolvePoolName() expected error, got nil")
					return
				}
				if tt.wantErrSubstr != "" && !containsString(err.Error(), tt.wantErrSubstr) {
					t.Errorf("ResolvePoolName() error = %v, want substring %q", err, tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolvePoolName() unexpected error = %v", err)
				return
			}
			if gotPool != tt.wantPool {
				t.Errorf("ResolvePoolName() pool = %q, want %q", gotPool, tt.wantPool)
			}
			if gotPrefix != tt.wantPrefix {
				t.Errorf("ResolvePoolName() prefix = %q, want %q", gotPrefix, tt.wantPrefix)
			}
		})
	}
}

func TestValidateRoutes(t *testing.T) {
	tests := []struct {
		name          string
		rules         []RouteRule
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name: "valid_rules",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
				{Prefix: "order:", Pool: "order-pool"},
				{Prefix: "", Pool: "default-pool"},
			},
			wantErr: false,
		},
		{
			name: "duplicate_prefix_different_pool",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool-a"},
				{Prefix: "order:", Pool: "order-pool"},
				{Prefix: "user:", Pool: "user-pool-b"},
			},
			wantErr:       true,
			wantErrSubstr: "duplicate prefix",
		},
		{
			name: "duplicate_prefix_same_pool",
			rules: []RouteRule{
				{Prefix: "user:", Pool: "user-pool"},
				{Prefix: "user:", Pool: "user-pool"},
			},
			wantErr:       true,
			wantErrSubstr: "duplicate prefix",
		},
		{
			name: "multiple_empty_prefixes",
			rules: []RouteRule{
				{Prefix: "", Pool: "default-a"},
				{Prefix: "", Pool: "default-b"},
			},
			wantErr:       true,
			wantErrSubstr: "duplicate prefix",
		},
		{
			name:    "empty_rules",
			rules:   []RouteRule{},
			wantErr: false,
		},
		{
			name: "single_empty_prefix",
			rules: []RouteRule{
				{Prefix: "", Pool: "default-pool"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoutes(tt.rules)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRoutes() expected error, got nil")
					return
				}
				if tt.wantErrSubstr != "" && !containsString(err.Error(), tt.wantErrSubstr) {
					t.Errorf("ValidateRoutes() error = %v, want substring %q", err, tt.wantErrSubstr)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateRoutes() unexpected error = %v", err)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestResolvePoolNameWithValidation(t *testing.T) {
	t.Run("validates_rules_first", func(t *testing.T) {
		rules := []RouteRule{
			{Prefix: "user:", Pool: "pool-a"},
			{Prefix: "user:", Pool: "pool-b"},
		}
		err := ValidateRoutes(rules)
		if err == nil {
			t.Error("expected validation error for duplicate prefix")
		}
		var valErr *ValidationError
		if !errors.As(err, &valErr) {
			t.Errorf("expected ValidationError, got %T", err)
		}
	})
}
