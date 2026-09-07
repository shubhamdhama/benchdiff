package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchSuiteArtifactIsolation(t *testing.T) {
	oldSuite := makeBenchSuite("old", "abc1234", "subject", []string{"FEATURE=false"}, false, true)
	newSuite := makeBenchSuite("new", "abc1234", "subject", []string{"FEATURE=true"}, false, true)

	if oldSuite.artDir == newSuite.artDir {
		t.Fatalf("old and new artifact directories must differ: %q", oldSuite.artDir)
	}
	if oldSuite.profilesDir() == newSuite.profilesDir() {
		t.Fatalf("old and new profile directories must differ: %q", oldSuite.profilesDir())
	}
	if got := testBinDir(oldSuite.ref, []string{"./pkg"}); got != testBinDir(newSuite.ref, []string{"./pkg"}) {
		t.Fatalf("same-ref suites should share binary directory: %q", got)
	}
}

func TestBenchSuiteLegacyArtifactDirectory(t *testing.T) {
	suite := makeBenchSuite("old", "abc1234", "subject", nil, false, false)
	if expected := testArtifactsDir(suite.ref); suite.artDir != expected {
		t.Fatalf("expected legacy artifact directory %q, got %q", expected, suite.artDir)
	}
}

func TestLogRunCommandIncludesEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run")
	if err := logRunCommand(path, []string{"FEATURE=true"}, []string{"./benchmark", "-test.bench", "."}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); !strings.HasPrefix(got, `"env" "FEATURE=true" "./benchmark"`) {
		t.Fatalf("run command does not include environment: %q", got)
	}
}
