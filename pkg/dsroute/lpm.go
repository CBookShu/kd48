package dsroute

import (
	"fmt"
	"strings"
)

type RouteRule struct {
	Prefix string
	Pool   string
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func ResolvePoolName(rules []RouteRule, routingKey string) (poolName, matchedPrefix string, err error) {
	var bestCandidate *RouteRule
	bestLen := -1

	for i := range rules {
		rule := &rules[i]
		effectiveLen := len(rule.Prefix)

		if rule.Prefix == "" {
			if bestLen < 0 {
				bestCandidate = rule
				bestLen = 0
			}
			continue
		}

		if strings.HasPrefix(routingKey, rule.Prefix) {
			if effectiveLen > bestLen {
				bestCandidate = rule
				bestLen = effectiveLen
			}
		}
	}

	if bestCandidate == nil {
		return "", "", fmt.Errorf("no matching route for key %q", routingKey)
	}

	return bestCandidate.Pool, bestCandidate.Prefix, nil
}

func ValidateRoutes(rules []RouteRule) error {
	seen := make(map[string]int)
	for i, rule := range rules {
		if firstIdx, exists := seen[rule.Prefix]; exists {
			return &ValidationError{
				Message: fmt.Sprintf("duplicate prefix %q at index %d and %d (pools: %q and %q)",
					rule.Prefix, firstIdx, i, rules[firstIdx].Pool, rule.Pool),
			}
		}
		seen[rule.Prefix] = i
	}
	return nil
}
