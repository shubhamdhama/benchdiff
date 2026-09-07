package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// parseEnvOverrides validates and normalizes repeatable KEY=VALUE arguments.
// Sorting makes equivalent configurations share the same artifact directory,
// regardless of the order in which their flags were provided.
func parseEnvOverrides(values []string) ([]string, error) {
	overrides := make(map[string]string, len(values))
	for _, value := range values {
		name, _, ok := strings.Cut(value, "=")
		if !ok || name == "" {
			return nil, errors.Errorf("invalid environment override %q; expected KEY=VALUE", value)
		}
		if strings.IndexByte(value, 0) >= 0 {
			return nil, errors.Errorf("invalid environment override %q; contains NUL", value)
		}
		if _, exists := overrides[name]; exists {
			return nil, errors.Errorf("environment variable %q specified more than once", name)
		}
		overrides[name] = value
	}

	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)

	normalized := make([]string, 0, len(names))
	for _, name := range names {
		normalized = append(normalized, overrides[name])
	}
	return normalized, nil
}

// artifactNamespace identifies one side and its runtime environment without
// exposing environment values in directory names.
func artifactNamespace(side string, env []string) string {
	if len(env) == 0 {
		return side + "-env-default"
	}
	sum := sha256.Sum256([]byte(strings.Join(env, "\x00")))
	return side + "-env-" + hex.EncodeToString(sum[:6])
}
