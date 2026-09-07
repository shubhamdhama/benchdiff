package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// capture executes the command specified by args and returns its stdout. If
// the process exits with a failing exit code, capture instead returns an error
// which includes the process's stderr.
func capture(args ...string) (string, error) {
	var cmd *exec.Cmd
	if len(args) == 0 {
		panic("capture called with no arguments")
	} else if len(args) == 1 {
		cmd = exec.Command(args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = errors.Errorf("%s: %s", err, exitErr.Stderr)
		}
		return "", err
	}
	return string(bytes.TrimSpace(out)), err
}

// spawn executes the command specified by args. The subprocess inherits the
// current processes's stdin, stdout, and stderr streams. If the process exits
// with a failing exit code, run returns a generic "process exited with
// status..." error, as the process has likely written an error message to
// stderr.
func spawn(args ...string) error {
	return spawnWith(os.Stdin, os.Stdout, os.Stderr, args...)
}

// spawnWith executes the command specified by args using the provided reader
// and writers for process I/O. The subprocess inherits the current processes's
// stdin, stdout, and stderr streams. If the process exits with a failing exit
// code, run returns a generic "process exited with status..." error, as the
// process has likely written an error message to stderr.
func spawnWith(in io.Reader, out, err io.Writer, args ...string) error {
	return spawnWithEnv(in, out, err, nil, args...)
}

// spawnWithEnv executes the command with the supplied environment overrides.
// The command continues to inherit all other variables from benchdiff.
func spawnWithEnv(in io.Reader, out, err io.Writer, env []string, args ...string) error {
	return spawnWithEnvContext(context.Background(), in, out, err, env, args...)
}

// spawnWithEnvContext executes the command with the supplied environment
// overrides and stops it when ctx is cancelled. The command continues to
// inherit all other variables from benchdiff.
func spawnWithEnvContext(
	ctx context.Context, in io.Reader, out, err io.Writer, env []string, args ...string,
) error {
	var cmd *exec.Cmd
	if len(args) == 0 {
		panic("spawn called with no arguments")
	} else if len(args) == 1 {
		cmd = exec.CommandContext(ctx, args[0])
	} else {
		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	}

	cmd.Env = commandEnv(os.Environ(), env)

	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = err
	return cmd.Run()
}

// commandEnv overlays overrides onto base and ensures that
// runtimecontentionstacks remains enabled for mutex profiles.
func commandEnv(base, overrides []string) []string {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, entry := range append(append([]string(nil), base...), overrides...) {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			merged[name] = entry
		}
	}

	envGodebug := ""
	if entry, ok := merged["GODEBUG"]; ok {
		_, envGodebug, _ = strings.Cut(entry, "=")
	}
	if envGodebug != "" {
		envGodebug += ","
	}
	envGodebug += "runtimecontentionstacks=1"
	merged["GODEBUG"] = "GODEBUG=" + envGodebug

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	env := make([]string, 0, len(names))
	for _, name := range names {
		env = append(env, merged[name])
	}
	return env
}
