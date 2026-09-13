# Voice2Canvas Production Runbook

## Host architecture

- **Host**: CT110 (`prod`, `10.0.20.110`)
- **Service**: `voice2canvas.service` (or `v2ui.service`) running as system user `v2ui:v2ui`
- **Port**: `8083` (direct HTTP backend listener at `http://10.0.20.110:8083`)
- **Health check**: `http://127.0.0.1:8083/healthz`
- **Paths**:
  - Releases: `/opt/v2ui/releases/<commit>`
  - Current symlink: `/opt/v2ui/current` -> `releases/<commit>`
  - Prior symlink: `/opt/v2ui/prior` -> `releases/<prior-commit>`
  - Runtime working state: `/opt/v2ui/runtime` and `/var/lib/v2ui`
  - Protected configuration: `/etc/v2ui/v2ui.env` (mode 0640, `root:v2ui`)
- **Edge relay**: Optional `v2ui-edge-proxy.socket` (port `18083` loopback relay forwarding to `10.0.20.110:8083`)

## Provisioning

Host provisioning is performed via `deploy/install-production.sh`:

```sh
sudo deploy/install-production.sh
```

This script idempotently creates the `v2ui` system account, required directories under `/opt/v2ui` and `/etc/v2ui`, installs systemd units, populates `/etc/v2ui/v2ui.env` if absent, reloads systemd, and enables `v2ui.service`.

Configure `/etc/v2ui/v2ui.env` with required API keys (`GEMINI_API_KEY`) using `sudoedit`.

## Continuous deployment via Forge

Deployments are driven by `.github/workflows/deploy.yml` calling Forge's reusable deploy workflow:

1. Triggered on published GitHub Releases or `workflow_dispatch` with `commit_sha`.
2. Validates commit signature, main ancestry, and CI passing status.
3. Builds release tarball via `bazel build //:release`.
4. Executes `deploy/deploy-release.sh <archive> 10.0.20.110` from CT160 using the `gha-v2ui` SSH key.
5. Stages release in `/opt/v2ui/releases/<commit>`, updates `current`/`prior` atomically, restarts `voice2canvas.service` (`v2ui.service`), and polls `/healthz` for 15 seconds. If verification fails, automatic rollback is triggered.

## Manual verification

```sh
deploy/verify-production.sh --host 10.0.20.110 --port 8083
deploy/verify-production.sh --systemd
```

## Rollback procedure

To manually roll back to the previous release:

```sh
sudo deploy/rollback-production.sh
sudo systemctl restart v2ui.service
deploy/verify-production.sh --port 8083 --systemd
```
