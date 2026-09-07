package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// benchRun describes one invocation of one benchmark binary. Keeping this
// independent of process paths lets runners execute the same logical run on
// either the local machine or a remote machine.
type benchRun struct {
	test         string
	runPattern   string
	benchTime    string
	cpuProfile   bool
	memProfile   bool
	mutexProfile bool
}

type benchCommand struct {
	env  []string
	args []string
}

func (r benchRun) profilesEnabled() bool {
	return r.cpuProfile || r.memProfile || r.mutexProfile
}

// benchRunner controls where benchmark binaries are staged and executed.
// Prepare must be called before Run. Implementations write benchmark output to
// the suite's existing output file and place profiles in profileRunDir so the
// existing merge and reporting paths remain local.
type benchRunner interface {
	Prepare(context.Context, ...*benchSuite) error
	Command(*benchSuite, benchRun) (benchCommand, error)
	Run(context.Context, *benchSuite, benchRun) error
}

type localBenchRunner struct{}

func (localBenchRunner) Prepare(context.Context, ...*benchSuite) error {
	return nil
}

func (localBenchRunner) Command(bs *benchSuite, run benchRun) (benchCommand, error) {
	return benchCommand{
		env: bs.env,
		args: bs.buildBenchArgs(
			run.test, run.runPattern, run.benchTime,
			run.cpuProfile, run.memProfile, run.mutexProfile,
		),
	}, nil
}

func (r localBenchRunner) Run(ctx context.Context, bs *benchSuite, run benchRun) error {
	cmd, err := r.Command(bs, run)
	if err != nil {
		return err
	}
	return spawnWithEnvContext(ctx, os.Stdin, bs.outFile, bs.outFile, cmd.env, cmd.args...)
}

type roachprodBenchRunner struct {
	cluster         string
	target          string
	remoteRoot      string
	log             io.Writer
	remoteSuiteDirs map[*benchSuite]string
	remoteBinDirs   map[*benchSuite]string
	stagedBinDirs   map[string]string
	runCount        map[*benchSuite]int
}

func newRoachprodBenchRunner(cluster string) (*roachprodBenchRunner, error) {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		return nil, errors.New("roachprod cluster name is empty")
	}
	if strings.Contains(cluster, ":") {
		return nil, errors.New("--roachprod expects a cluster name without a node selector")
	}

	runID := fmt.Sprintf("%s-%d", time.Now().UTC().Format("20060102T150405Z"), os.Getpid())
	return &roachprodBenchRunner{
		cluster:         cluster,
		target:          cluster + ":1",
		remoteRoot:      path.Join("benchdiff", runID),
		log:             os.Stderr,
		remoteSuiteDirs: make(map[*benchSuite]string),
		remoteBinDirs:   make(map[*benchSuite]string),
		stagedBinDirs:   make(map[string]string),
		runCount:        make(map[*benchSuite]int),
	}, nil
}

func (r *roachprodBenchRunner) Prepare(ctx context.Context, suites ...*benchSuite) error {
	remoteBinariesDir := path.Join(r.remoteRoot, "binaries")
	r.logf("preparing remote benchmark target %s", r.target)
	r.logf("creating remote workspace %s", r.remoteRoot)
	if err := r.roachprod(ctx, os.Stdout, os.Stderr, "run", r.target, "--", "mkdir", "-p", remoteBinariesDir); err != nil {
		return errors.Wrap(err, "creating remote benchdiff directory")
	}

	for _, bs := range suites {
		if _, ok := r.remoteSuiteDirs[bs]; ok {
			continue
		}
		if bs.side == "" {
			return errors.New("benchmark suite has no side")
		}
		if bs.binDir == "" {
			return errors.Errorf("benchmark suite %q has not been built", bs.side)
		}

		remoteSuiteDir := path.Join(r.remoteRoot, bs.side)
		if err := r.roachprod(
			ctx, os.Stdout, os.Stderr, "run", r.target, "--", "mkdir", "-p", remoteSuiteDir,
		); err != nil {
			return errors.Wrapf(err, "creating remote directory for %s", bs.side)
		}
		remoteBinDir, staged := r.stagedBinDirs[bs.binDir]
		if !staged {
			remoteBinDir = path.Join(remoteBinariesDir, bs.side)
			r.logf("uploading %s benchmark binaries to %s", bs.side, remoteBinDir)
			if err := r.roachprod(
				ctx, os.Stdout, os.Stderr, "put", r.target, bs.binDir, remoteBinDir,
			); err != nil {
				return errors.Wrapf(err, "staging %s benchmark binaries", bs.side)
			}
			r.stagedBinDirs[bs.binDir] = remoteBinDir
		} else {
			r.logf("reusing uploaded binaries for %s from %s", bs.side, remoteBinDir)
		}
		r.remoteSuiteDirs[bs] = remoteSuiteDir
		r.remoteBinDirs[bs] = remoteBinDir
	}
	r.logf("remote benchmark target is ready")
	return nil
}

func (r *roachprodBenchRunner) Command(bs *benchSuite, run benchRun) (benchCommand, error) {
	return r.command(bs, run, r.runCount[bs]+1)
}

func (r *roachprodBenchRunner) command(
	bs *benchSuite, run benchRun, runNumber int,
) (benchCommand, error) {
	remoteSuiteDir, ok := r.remoteSuiteDirs[bs]
	if !ok {
		return benchCommand{}, errors.Errorf("benchmark suite %q has not been staged", bs.side)
	}
	remoteBinDir, ok := r.remoteBinDirs[bs]
	if !ok {
		return benchCommand{}, errors.Errorf("benchmark suite %q has no staged binaries", bs.side)
	}

	remoteProfileDir := path.Join(
		remoteSuiteDir, "profiles", fmt.Sprintf("run-%d", runNumber),
	)
	remoteBin := path.Join(remoteBinDir, run.test)
	args := bs.buildBenchArgsAt(
		run.test, remoteBin, remoteProfileDir, run.runPattern, run.benchTime,
		run.cpuProfile, run.memProfile, run.mutexProfile,
	)
	remoteArgs := []string{"roachprod", "run", r.target, "--", "env"}
	// Only explicit benchmark environment overrides are sent to the VM. The
	// local process environment may contain credentials and machine-specific
	// settings that must not become part of a remote benchmark.
	remoteArgs = append(remoteArgs, commandEnv(nil, bs.env)...)
	remoteArgs = append(remoteArgs, args...)
	return benchCommand{args: remoteArgs}, nil
}

func (r *roachprodBenchRunner) Run(ctx context.Context, bs *benchSuite, run benchRun) error {
	if bs.outFile == nil {
		return errors.Errorf("benchmark suite %q has no output file", bs.side)
	}

	r.runCount[bs]++
	cmd, err := r.command(bs, run, r.runCount[bs])
	if err != nil {
		return err
	}
	remoteSuiteDir := r.remoteSuiteDirs[bs]
	remoteProfileDir := path.Join(
		remoteSuiteDir, "profiles", fmt.Sprintf("run-%d", r.runCount[bs]),
	)
	if run.profilesEnabled() {
		if err := r.roachprod(
			ctx, os.Stdout, os.Stderr, "run", r.target, "--", "mkdir", "-p", remoteProfileDir,
		); err != nil {
			return errors.Wrap(err, "creating remote profile directory")
		}
	}

	runErr := spawnWithEnvContext(ctx, os.Stdin, bs.outFile, bs.outFile, cmd.env, cmd.args...)
	if !run.profilesEnabled() {
		return runErr
	}
	if err := os.MkdirAll(bs.profileRunDir(), 0744); err != nil {
		return err
	}
	remoteProfiles := path.Join(remoteProfileDir, "*")
	r.logf("retrieving %s profiles from %s", bs.side, remoteProfileDir)
	if err := r.roachprod(
		ctx, os.Stdout, os.Stderr, "get", r.target, remoteProfiles, bs.profileRunDir(),
	); err != nil {
		if runErr != nil {
			return errors.Wrapf(err, "benchmark command also failed: %v", runErr)
		}
		return errors.Wrap(err, "retrieving remote profiles")
	}
	return runErr
}

func (r *roachprodBenchRunner) logf(format string, args ...interface{}) {
	fmt.Fprintf(r.log, "benchdiff: "+format+"\n", args...)
}

func (r *roachprodBenchRunner) roachprod(
	ctx context.Context, stdout, stderr io.Writer, args ...string,
) error {
	return spawnWithEnvContext(
		ctx, os.Stdin, stdout, stderr, nil, append([]string{"roachprod"}, args...)...,
	)
}

var _ benchRunner = localBenchRunner{}
var _ benchRunner = (*roachprodBenchRunner)(nil)
