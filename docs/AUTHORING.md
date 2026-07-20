# Authoring Obacht Templates (spec v2.8)

This document is for community template authors. The spec lives at
`schema/manifest-v2.json`; this is the human-readable companion.

## What a template is

A declarative description of one workload (one website, one tool, one
service), one ingress contract, and optional configuration. Obacht's
agent reads it, runs it on a Pi, and Caddy makes it reachable on a
domain. The user never touches Docker, ports, or env files directly.

## Spec revision

Every manifest must declare the spec revision it targets:

```yaml
spec:
  minSpecVersion: "v2.1"
```

The agent ships an embedded `SupportedSpecVersion` constant. Manifests
asking for a higher revision are refused. v2.1 is the first version
where `minSpecVersion` is mandatory.

Revisions since v2.1 are **additive** — older agents ignore the new
fields, so you only need to raise `minSpecVersion` if your template
actually depends on one:

- **v2.2** — `immutable` on config/secret fields (first-boot bootstrap values).
- **v2.3** — `allowUnpinnedImages` + `envConfigKey` on compose; `${cfg.X}`
  placeholders for compose `primaryPort` and service `targetPort`.
- **v2.4** — macOS platform: the `mac` device, the `darwin/arm64`
  architecture, `compatibility.excludeDevices`, and the `system` runtime's
  `host_service` flavor (a launchd-managed host binary).
- **v2.5** — `spec.gettingStarted`: an optional post-install note shown to
  the user after install (e.g. "open the app and register your account").
  Informational only; the agent never sees it.
- **v2.6** — typed config-field renderers `timezone`, `email`, `domain`. These
  are render-only: the webapp shows a better input widget (e.g. a searchable
  IANA-timezone picker), but the value resolves to the same string a `text`
  field would. The agent and api need no change, so you do **not** raise
  `minSpecVersion` just to use them — the registry validator is what gates them.
- **v2.7** — optional `advanced: true` on a config field. Easy-Mode clients hide
  the field (the install falls back to its `default`); Advanced Mode shows it.
  Render-only (agent/api ignore it). **Only flag fields that have a usable
  `default` and aren't required-without-default** — otherwise an Easy-Mode
  install ends up with no value. Don't raise `minSpecVersion` for it.
- **v2.8** — Pi system-runtime flavors + device-feature gating + inventory
  selects (see "System templates" below). Adds
  `system.managed_service` (digest-pinned host binary run as an agent-generated
  hardened systemd unit), the `system.kiosk` marker,
  `compatibility.requiresFeatures`, and `configField.optionsSource`. v2.8 also
  **withdraws** the never-shipped free-form systemd flavor
  (`unitName`/`unitTemplate`): system templates never author unit text — the
  registry rejects manifests using it.

## Choosing a runtime

| Runtime    | When to use                                                                               |
|------------|--------------------------------------------------------------------------------------------|
| `container`| Single all-in-one image. Pocketbase, Memos, Vaultwarden, Gitea-with-SQLite. Smallest blast radius. |
| `compose`  | Multi-container bundle — needs a separate database, cache, queue, etc. Each bundle is fully isolated; **never share infra across bundles**. |
| `system`   | Cannot run in Docker (kiosk, hardware-attached). Needs Power Mode unlocked on the device. See "System templates" below. |

## System templates (v2.8, Pi)

System templates run **on the host**, outside Docker. They are the sanctioned
escape hatch the compose allowlist points to for device/hardware access — and
they are deliberately more constrained than they look:

- **Official only.** `runtime.type: system` is rejected on the community
  submission path; only obacht-signed templates ship it.
- **`minSudoLevel: power` is mandatory** — the agent refuses the install while
  Power Mode is locked, and only advertises the `runtime.system` capability
  with Power Mode on.
- **You never write a systemd unit.** Declare a `managed_service` (binary +
  pinned digest + argv + env + hardware grants); the agent generates a hardened
  unit (`DynamicUser`, `DevicePolicy=closed` + your declared `DeviceAllow`,
  `NoNewPrivileges`, `ProtectSystem=strict`) and the root helper independently
  re-validates it before installing. The binary name must be on the agent's
  closed allowlist and the download host on its host allowlist — adding either
  is an agent release, not a manifest change.
- **`kiosk: {}`** is a marker flavor: the whole kiosk session (compositor +
  Chromium) is agent-shipped; the template contributes `configSchema` and
  `files` only. Pair it with `exclusivityGroup: display-output` and
  `requiresFeatures: [desktop-chromium, wayland-compositor]`.
- **`files[]`** write inline content to the instance-scoped paths
  `/etc/obacht/svc/<instanceID>/` or `/var/lib/obacht/svc/<instanceID>/`
  (kiosk: `/etc/obacht/kiosk/`) — nothing else.
- **`requiresFeatures`** gates the template on detected device features
  (`desktop-chromium`, `wayland-compositor`, `csi-or-usb-camera`). Enforced by
  the api compat check, the client catalog filter and the on-device install
  assertion.
- **Exposure stays HTTP-only** via `spec.services` (`targetType: host_port`) →
  device Caddy. `listen_ports` is documentation/validation, never a public port.
- **`optionsSource`** (select fields) fills options from device-reported
  inventory (`cameras`), e.g. picking the detected CSI/USB camera. Mutually
  exclusive with `options`; clients block the install while the inventory is
  empty.

## Compatibility (mandatory)

```yaml
spec:
  compatibility:
    devices: [raspberry-pi-4, raspberry-pi-5]   # optional allowlist
    architectures: [linux/arm64, linux/amd64]   # mandatory
    os:
      - id: raspberry-pi-os
        minVersion: "12"
    resources:
      minRamMb: 768
      minDiskMb: 2048
```

Be honest. A Pi 4/2GB has ~1.4GB usable after the OS — a template that
claims `minRamMb: 1024` is effectively excluding that device. Bundles
should always set `resources` (registry warns on values < 256MB).

## The compose allowlist

The compose body is YAML that **looks like** Docker Compose but is
parsed and validated against an obacht-specific allowlist before signing.

**Allowed top-level keys:** `services`, `volumes`, `networks` (intra-bundle naming only), `version`.

**Allowed per-service keys:** `image`, `command`, `entrypoint`,
`environment`, `volumes` (named volumes only), `depends_on`,
`healthcheck`, `restart` (`unless-stopped` | `on-failure` | `no`),
`labels`, `networks`, `tmpfs`, `read_only`, `user`, `working_dir`,
`cap_drop`, `security_opt` (`no-new-privileges:true` only),
`stop_grace_period`, `stop_signal`, `sysctls`, `shm_size`, `mem_limit`,
`cpus`, `init`.

**Forbidden — registry rejects on publish:**

| Key                  | Why                                                          |
|----------------------|---------------------------------------------------------------|
| `build`              | Templates ship images, never build on the Pi.                 |
| `network_mode`       | Bundle isolation requires obacht-managed networks.            |
| `privileged`         | Defeats sandboxing.                                           |
| `cap_add`, `devices` | Closed list — use `system` runtime + Power Mode if required. |
| `pid`, `ipc`, `uts`  | Namespace bypass.                                             |
| `host bind mounts`   | Use named volumes.                                            |
| `ports` (host:c)     | Ingress is owned by the device's Caddy via `spec.services`.   |
| `secrets`, `configs` | Use `${secret.x}` / `${cfg.x}`.                               |
| `extends`            | No external compose files.                                    |
| `profiles`           | All declared services must always run.                        |
| `x-*`                | Only `x-obacht-data` is allowed.                              |

The only obacht-defined extension today is **`x-obacht-data: true`** on
a top-level volume — marks that volume as user-data. Such volumes
survive uninstall (renamed to `obacht-archived-...`, never deleted).

## Image pinning

In `runtime.compose.imageDigests` the registry will populate a map from
each `services.<name>.image` reference to its `sha256:...` digest. You
write `imageDigests: {}` (empty); the publish pipeline fills it.

The agent rewrites every `image:` line to `image: ref@sha256:...`
before `docker compose up`. A manifest signature without image-pinning
would only protect the YAML, not the running code — that's why this is
the trust boundary.

When an upstream image gets a new release, publish a new manifest
version with bumped `metadata.version`; the registry re-pins automatically.

## Configuration UX (`configSchema`)

Only ask the user for things a non-technical user can decide. Internal
hostnames, ports, connection strings: hardcoded inside the bundle.

Field types:

- `text` — freeform string
- `number` — integer
- `select` — fixed dropdown options
- `boolean` — toggle
- `secret` — masked, set-once, cannot be retrieved later
- `service_reference` — points at another obacht template (v2.1 renders
  as a normal text input + note "auto-discovery in a future release")
- `timezone` (v2.6) — IANA timezone; webapp renders a searchable picker.
  Prefer this over a `text` field with an "e.g. Europe/Berlin" hint.
- `email` (v2.6) — single email address; webapp renders an email input.
- `domain` (v2.6) — bare hostname (no scheme/path); webapp normalises and
  validates. Use for "the domain you attach under Hosting" fields. For a full
  URL incl. `https://`, keep `text` for now (a `url` type is planned).

## Secrets — auto-generated, never leave the Pi

For database passwords, signing keys, and other obacht-generated
material:

```yaml
spec:
  secretsSchema:
    - key: db_root_password
      length: 32
      charset: alphanumeric
    - key: app_session_secret
      length: 64
      charset: hex
```

Reference in the compose body via `${secret.db_root_password}`. The
agent generates each value with `crypto/rand` on first install,
persists it to `/var/lib/obacht/agent/secrets.db` (mode 0600), and
never sends it over the network. The api never sees these. The webapp
has no UI to read them.

Also list the corresponding env var keys in `spec.secrets` (uppercase)
to ensure they get redacted from logs and audit records.

## Ingress contract (`services`)

```yaml
services:
  - name: web
    targetType: container_port
    targetService: web              # required for compose runtime
    targetPort: 80
```

For compose: `targetService` must reference a service in the body. Only
services listed here are reachable from the public domain — internal
databases/caches must not appear here.

## Interfaces (`provides` / `consumes`) — declared, not enforced

Use this to mark services that participate in cross-bundle wiring. In
v2.1 the agent ignores both blocks; phase 2 makes resolution real.
Writing your manifest with these blocks today means you don't have to
re-publish later.

```yaml
spec:
  provides:
    - interface: openai_compatible
      version: v1
      service: ollama
      port: 11434
      path: /v1
  consumes:
    - interface: openai_compatible
      version: v1
      configKey: llm_endpoint
```

Reserved interfaces: `openai_compatible`, `s3_compatible`,
`smtp_relay`, `postgres_database`, `oidc_provider` (last one is
reserved for Spaces — don't claim it yet).

## Update strategy

Every published manifest must bump `metadata.version` (semver). The
registry rejects republishes of the same `name@version`.

- **Patch** for image-digest-only updates
- **Minor** for added config fields
- **Major** for breaking config changes (registry warns; agent does not refuse)

## Worked examples

- [`examples/whoami.yml`](../examples/whoami.yml) — simplest container
- [`examples/uptime-kuma.yml`](../examples/uptime-kuma.yml) — single container with persistent volume
- [`examples/etherpad.yml`](../examples/etherpad.yml) — single container with embedded DB
- [`examples/wordpress-mysql.yml`](../examples/wordpress-mysql.yml) — compose bundle (web + DB) with secrets
- [`examples/llm-stack.yml`](../examples/llm-stack.yml) — compose bundle providing the `openai_compatible` interface
- [`examples/openwebui.yml`](../examples/openwebui.yml) — single container consuming `openai_compatible`
