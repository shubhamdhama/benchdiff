package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseEnvOverrides(t *testing.T) {
	testCases := []struct {
		name     string
		values   []string
		expected []string
		err      string
	}{
		{
			name:     "normalizes order",
			values:   []string{"SECOND=two=parts", "EMPTY=", "FIRST=one"},
			expected: []string{"EMPTY=", "FIRST=one", "SECOND=two=parts"},
		},
		{
			name:   "missing value separator",
			values: []string{"FIRST"},
			err:    "expected KEY=VALUE",
		},
		{
			name:   "missing name",
			values: []string{"=value"},
			err:    "expected KEY=VALUE",
		},
		{
			name:   "duplicate name",
			values: []string{"FIRST=one", "FIRST=two"},
			err:    "specified more than once",
		},
		{
			name:   "NUL",
			values: []string{"FIRST=one\x00two"},
			err:    "contains NUL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := parseEnvOverrides(tc.values)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("expected error containing %q, got %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestCommandEnv(t *testing.T) {
	base := []string{
		"UNCHANGED=value",
		"OVERRIDDEN=old",
		"GODEBUG=existing=1",
	}
	overrides := []string{
		"OVERRIDDEN=new",
		"ADDED=value",
		"GODEBUG=override=1",
	}
	expected := []string{
		"ADDED=value",
		"GODEBUG=override=1,runtimecontentionstacks=1",
		"OVERRIDDEN=new",
		"UNCHANGED=value",
	}

	if actual := commandEnv(base, overrides); !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

func TestArtifactNamespace(t *testing.T) {
	env := []string{"FIRST=one", "SECOND=two"}
	if old, new := artifactNamespace("old", env), artifactNamespace("new", env); old == new {
		t.Fatalf("old and new artifact namespaces must differ: %q", old)
	}
	if first, second := artifactNamespace("old", env), artifactNamespace("old", env); first != second {
		t.Fatalf("artifact namespace is not stable: %q != %q", first, second)
	}
	if strings.Contains(artifactNamespace("old", env), "one") {
		t.Fatal("artifact namespace exposes environment values")
	}
}
