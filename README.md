# benchdiff

A tool for automating the process of running and comparing Go benchmarks across code changes.

## Usage

```
$ benchdiff --help
usage: benchdiff [--old <commit>] [--new <commit>] [--old-env <key=value>] [--new-env <key=value>] <pkgs>...

benchdiff automates the process of running and comparing Go microbenchmarks
across code changes and runtime configurations.

benchdiff runs all microbenchmarks in the specified packages against the old and
new commit or environment configuration. It then passes the benchmark output
through benchstat to compute statistics about the results. Environment options
override inherited variables only while running benchmark binaries, not while
building them.

By default, benchdiff outputs these results in a textual format. However, if the
--sheets flag is passed then it will upload the result to a Google Sheets
spreadsheet. To access this, users must have a Google service account. For
information, see https://cloud.google.com/iam/docs/service-accounts.

The Google service account must meet the following conditions:
1. The Google Sheets API must be enabled for the account's project
2. The Google Drive  API must be enabled for the account's project

When the --sheets flag is passed, benchdiff will search for a credentials file
containing the service account key using the GOOGLE_APPLICATION_CREDENTIALS
environment variable. See https://cloud.google.com/docs/authentication/production.

Options:
  -n, --new       <commit> measure the difference between this commit and old (default HEAD)
  -o, --old       <commit> measure the difference between this commit and new (default new~)
      --new-env   <key=val> set an environment variable when running new; repeatable
      --old-env   <key=val> set an environment variable when running old; repeatable
                           with environment flags but no commits, compare HEAD to itself
  -r, --run       <regexp> run only benchmarks matching regexp
  -c, --count     <n>      run tests and benchmarks n times (default 10)
  -t, --threshold <n>      exit with code 0 if all regressions are below threshold, else 1
      --post-checkout      an optional command to run after checking out each branch to
                           configure the git repo so that 'go build' succeeds
      --roachprod          run benchmarks on node 1 of an existing roachprod cluster
      --csv                output the results in a csv format
      --html               output the results in an HTML table
      --sheets             output the results to a new Google Sheets document
      --help               display this help

Example invocations:
  $ benchdiff --sheets ./pkg/...
  $ benchdiff --old=master~ --new=master --threshold=0.2 ./pkg/kv ./pkg/storage/...
  $ benchdiff --old-env=FEATURE=false --new-env=FEATURE=true ./pkg/kv/...
  $ benchdiff --new=d1fbdb2 --run=Datum --count=2 --csv ./pkg/sql/...
  $ benchdiff --roachprod=user-bench ./pkg/util/uuid
  $ benchdiff --new=6299bd4 --sheets --post-checkout='dev generate go' ./pkg/workload/...
```

## Examples

Comparing two runtime configurations of the current commit:

```
$ benchdiff \
    --old-env=COCKROACH_TEST_DRPC=false \
    --new-env=COCKROACH_TEST_DRPC=true \
    ./pkg/kv/kvserver
```

Both sides use the same binary built from `HEAD`. Specify `--old` or `--new` to
combine environment overrides with the usual commit comparison. Repeat an
environment option to override multiple variables.

Running benchmarks on an existing roachprod cluster:

```
$ benchdiff -b \
    --roachprod=shubham-bench \
    --old-env=FEATURE=false \
    --new-env=FEATURE=true \
    --cpuprofile --memprofile --mutexprofile \
    ./pkg/util/uuid
```

`benchdiff` builds binaries on the local Linux machine, uploads them to node 1,
and runs the old and new suites there. The local machine and VM must therefore
have compatible operating systems and architectures. The `roachprod` command
must be available in `PATH`, and the cluster must already exist. Benchmark
output and profiles are copied into the same local `benchdiff/<ref>/artifacts`
directories used by local runs. When both suites share a binary, as in an
environment-only comparison, it is uploaded only once.

Using text output:

```
$ benchdiff --new=6299bd4 ./pkg/workload/...
test binaries already exist for 'efcf66c'; skipping build
test binaries already exist for '6299bd4'; skipping build

running benchmarks:
  pkg=1/7 iter=2/2 cockroachdb/cockroach/pkg/workload -
  pkg=2/7 iter=2/2 cockroachdb/cockroach/pkg/workload/bank /
  pkg=3/7 iter=2/2 cockroachdb/cockroach/pkg/workload/faker |
  pkg=4/7 iter=2/2 cockroachdb/cockroach/pkg/workload/movr |
  pkg=5/7 iter=2/2 cockroachdb/cockroach/pkg/workload/tpcc /
  pkg=6/7 iter=2/2 cockroachdb/cockroach/pkg/workload/workloadsql \
  pkg=7/7 iter=2/2 cockroachdb/cockroach/pkg/workload/ycsb |

name                             old time/op    new time/op     delta
InitialData/tpcc/warehouses=1-8     304ms ± 4%      195ms ± 1%  -35.61%  (p=0.008 n=5+5)
InitialData/bank/rows=1000-8        281µs ± 3%      282µs ± 2%     ~     (p=0.548 n=5+5)
CSVRowsReader-8                    17.2µs ± 1%     17.5µs ± 0%   +1.80%  (p=0.016 n=5+4)
WriteCSVRows-8                     14.8µs ± 3%     15.6µs ± 5%   +5.28%  (p=0.032 n=5+5)

name                             old speed      new speed       delta
InitialData/tpcc/warehouses=1-8   363MB/s ± 4%    563MB/s ± 1%  +55.23%  (p=0.008 n=5+5)
CSVRowsReader-8                  98.1MB/s ± 1%  101.0MB/s ± 0%   +2.96%  (p=0.016 n=5+4)
InitialData/bank/rows=1000-8      413MB/s ± 3%    411MB/s ± 2%     ~     (p=0.548 n=5+5)
WriteCSVRows-8                    113MB/s ± 3%    113MB/s ± 5%     ~     (p=1.000 n=5+5)

name                             old alloc/op   new alloc/op    delta
InitialData/tpcc/warehouses=1-8     128kB ± 0%       81kB ± 0%  -36.19%  (p=0.008 n=5+5)
InitialData/bank/rows=1000-8       19.1kB ± 0%     19.1kB ± 0%     ~     (all equal)
WriteCSVRows-8                     5.70kB ± 0%     5.70kB ± 0%     ~     (all equal)
CSVRowsReader-8                    7.38kB ± 0%     7.38kB ± 0%     ~     (all equal)

name                             old allocs/op  new allocs/op   delta
InitialData/tpcc/warehouses=1-8       587 ± 0%        583 ± 0%   -0.61%  (p=0.008 n=5+5)
InitialData/bank/rows=1000-8        1.02k ± 0%      1.02k ± 0%     ~     (all equal)
WriteCSVRows-8                       50.0 ± 0%       50.0 ± 0%     ~     (all equal)
CSVRowsReader-8                      55.0 ± 0%       55.0 ± 0%     ~     (all equal)
```

Using Google Sheets output:

```
$ export GOOGLE_APPLICATION_CREDENTIALS=/Users/nathan/.service-account-creds.json
$ benchdiff --new=6299bd4 --sheets ./pkg/workload/...
test binaries already exist for 'efcf66c'; skipping build
test binaries already exist for '6299bd4'; skipping build

running benchmarks:
  pkg=1/7 iter=2/2 cockroachdb/cockroach/pkg/workload -
  pkg=2/7 iter=2/2 cockroachdb/cockroach/pkg/workload/bank /
  pkg=3/7 iter=2/2 cockroachdb/cockroach/pkg/workload/faker |
  pkg=4/7 iter=2/2 cockroachdb/cockroach/pkg/workload/movr |
  pkg=5/7 iter=2/2 cockroachdb/cockroach/pkg/workload/tpcc /
  pkg=6/7 iter=2/2 cockroachdb/cockroach/pkg/workload/workloadsql \
  pkg=7/7 iter=2/2 cockroachdb/cockroach/pkg/workload/ycsb |

generated sheet: https://docs.google.com/spreadsheets/d/...
```
