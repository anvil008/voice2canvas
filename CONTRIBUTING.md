# Contributing

Issues and focused pull requests are welcome. Keep protocol changes reflected
in `PROTOCOL.md`, add tests for both accepted and rejected frames, and never
commit credentials or captured user audio.

## Build boundary

Bazel is the only build entry point. `//:ci` and `//:release` are both filegroups over the
single `//:pipeline` action (`build/bazel/ci.sh`), which emits `dist/v2ui-release.tar.gz`
and its `.sha256`; the Tier-2 per-component split that nexus and swarm have has not been
done here yet. Before opening a pull request, run the whole gate the way CI runs it:

```sh
forge-bazel build //:ci --config=agent
```

`forge-bazel` rsyncs the working tree to the CT160 build runner and builds there as
`gha-v2ui`; do not run the Go or Vite gates directly on the development host. GitHub CI
runs the same targets on CT160 with `--config=ci`. `ci` and `agent` are the only configs
in `.bazelrc`, and it does not yet set `--disk_cache=/var/cache/bazel/disk/v2ui` or
`--repository_cache=/var/cache/bazel/repo` — CT160's `/etc/bazel.bazelrc` deliberately
owns no cache paths, so caching is still only the per-repo output base. CI feeds the Bazel
build event protocol through `forge/tools/bazel-metrics` into Alloy/Loki; the "Bazel CI"
Grafana dashboard is where duration and cache-hit rate are read.

The underlying per-language commands still exist for tight local loops, but they are not
the gate:

```sh
cd frontend && npm test && npm run build
cd ../backend && go test ./... && go vet ./...
```

## Git workflow

Trunk is `main`, protected by the `main-trunk` ruleset — every change lands through a pull
request, history stays linear, commits are SSH-signed, and force-push and deletion are
blocked. The required status check is `CI / Quality and integration gates`, which is
`.github/workflows/ci.yml` calling the Forge CI template,
`anvil008/forge/.github/workflows/ci.yml@main`, on pull requests and on push to `main`. A
push to `main` runs CI and never touches production.

Production ships by publishing a GitHub Release (dated tags, `vYYYY.MM.DD[.N]`;
pre-releases are skipped), with `workflow_dispatch` on the same deploy workflow for
redeploy, rollback and emergencies. Both triggers feed one job that re-verifies the
resolved commit before building — an ancestor of `origin/main`, signed by a key in
`.github/signing-keys/anvil.allowed_signers`, and carrying a successful
`CI / Quality and integration gates`. The job declares the `production` environment, but
on GitHub Pro that environment is an audit label rather than a reviewer gate, so
publishing the Release is the human action that ships (forge
`docs/adr/0001-ci-standardization.md`).
