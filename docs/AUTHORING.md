# Authoring Obacht Templates (spec v2.9)

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
- **v2.9** — `configField.optionRequirements` + `spec.hosting`. See "Per-option
  resources" and "Hosting hints" below. Both additive; don't raise
  `minSpecVersion` for them (the registry validator gates them, agents that
  predate v2.9 simply fall back to the template-wide `minDiskMb` floor).
  v2.9 also adds `services[].readiness` (tunes the post-install HTTP probe
  behind the instance's `service_ready` state), `host_service.post_start`
  (loopback HTTP hooks the agent runs after a macOS host service is up) and
  the `${host.data}` placeholder for host services. See "Readiness" and
  "macOS host services" below. Older agents ignore the two blocks and never
  resolve the placeholder, so a template whose install depends on them (an
  Ollama pull via `post_start`, a data directory via `${host.data}`) sets
  `minSpecVersion: "v2.9"`. A template that only tunes `readiness` can stay
  on its current revision; old agents simply keep reporting the generic
  install state.

## Per-option resources (v2.9)

Some templates offer a choice whose footprint differs by an order of
magnitude, e.g. which OpenStreetMap region to import. Declare the minimum
per option instead of publishing one template per region:

```yaml
spec:
  compatibility:
    resources: { minRamMb: 8192, minDiskMb: 40000 }   # floor = smallest option
  configSchema:
    - key: region
      label: Region
      type: select
      default: de
      options:
        - { value: de,   label: Germany }
        - { value: dach, label: Germany, Austria, Switzerland }
        - { value: eu,   label: Europe }
      optionRequirements:
        de:   { minRamMb: 8192,  minDiskMb: 40000 }
        dach: { minRamMb: 8192,  minDiskMb: 50000 }
        eu:   { minRamMb: 16384, minDiskMb: 200000 }
```

Rules: only on `type: select` with static `options`; every key must be an
`options[].value`; every value must be ≥ `compatibility.resources`. Options
without an entry inherit the floor. Clients grey out options the device can't
satisfy (RAM from the agent identity, free disk from live telemetry), the api
rejects an install whose chosen option the device fails, and the agent's disk
preflight requires the chosen option's `minDiskMb` before pulling.

Be honest about disk: count the download, the temporary files during import
**and** the final data. A 4.5 GB PBF that becomes a 25 GB database needs
`minDiskMb` around 40000, not 25000.

## Hosting hints (v2.9, render-only)

Basic Auth is a property of the domain the user attaches under Hosting
(agent capability `ingress.basic-auth`), never of the template. A template
can still say that its instances should normally sit behind one:

```yaml
spec:
  hosting:
    recommendBasicAuth: true
    basicAuthReason: "An open geocoding API on a home connection is easy to overload."
```

Easy-Mode clients pre-enable the Basic Auth toggle when attaching a domain
and show the reason; Advanced flows may show the hint but must not force it.
The agent and api ignore the block. Use it for open APIs without their own
login (geo services, model runners), not for apps with an account system.

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
  `/etc/obacht/svc/<instanceID>/` or `/opt/obacht/svc/<instanceID>/`
  (kiosk: `/etc/obacht/kiosk/`) — nothing else. **These files are
  world-readable** (the workload runs as a transient systemd `DynamicUser`
  that must be able to read them), so **never put a secret in a
  `managed_service` `files[]` body** — use `secretsSchema` + env instead. A
  control character in a substituted `${cfg.X}` value is rejected (it would
  inject a structural line into the rendered file).
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

## macOS host services (v2.4) and post-start hooks (v2.9)

On a Mac the agent runs containers inside a VM, and some workloads need
the host instead (Ollama wants Metal and the machine's whole RAM). The
`host_service` flavor of the `system` runtime describes such a workload as
a structured launchd job: a pinned, checksum-verified binary plus argv and
env. You never write a plist. Host services are Mac-only; pair them with
`compatibility.devices: [mac]` and `architectures: [darwin/arm64]`.

```yaml
spec:
  runtime:
    type: system
    system:
      host_service:
        kind: ollama
        binary: ollama
        archive: tgz
        binary_url: https://github.com/ollama/ollama/releases/download/v0.40.0/ollama-darwin.tgz
        binary_digest: sha256:b490b4925a95c5f3dfcd889e566cf3dcd727848d59057fb00b03f1d6630326dc
        args: [serve]
        env:
          OLLAMA_HOST: "0.0.0.0:11434"
          OLLAMA_MODELS: "${host.data}/models"
        post_start:
          - http:
              method: POST
              url: http://127.0.0.1:11434/api/pull
              body: '{"name": "${cfg.model}", "stream": false}'
              timeoutSeconds: 3600
```

### Host placeholders

Besides `${cfg.X}` and `${instance.id}`, two placeholders are resolved by
the agent on the device at apply time, because only the device knows the
values. The api leaves them untouched when it materializes the install.

- `${host.gateway}` (v2.4): the IP of the VM's gateway, which is how a
  container inside the VM reaches a service on the Mac host. Used on the
  container side, e.g. `OLLAMA_BASE_URL: "http://${host.gateway}:11434"` in
  the Open WebUI template that talks to a host Ollama.
- `${host.data}` (v2.9): the managed data directory of this host-service
  instance. The agent creates it before the service starts (on macOS
  `~/Library/Application Support/obacht/host/data/<launchd label>`) and
  removes it with the instance. Allowed in `host_service.env` values and in
  `host_service.args`. Use it for everything the service writes, so an
  uninstall leaves nothing behind in the user's home directory. Without it
  Ollama keeps models in `~/.ollama`, where the next install finds stale
  leftovers.

### Post-start hooks

`post_start` lists HTTP requests the agent sends once the service is up.
Each entry has exactly one key, `http`, with `method` (`GET` or `POST`),
`url`, and optionally `body`, `contentType` (default `application/json`) and
`timeoutSeconds` (default 1800, allowed 10 to 7200; a model pull can take a
while). `url` and `body` may contain `${cfg.X}` placeholders.

How the agent runs them: after launchd has loaded the service, the agent
waits (retrying for up to 120 seconds) until the first hook's URL accepts a
TCP connection. Then it runs the hooks in order. A hook succeeds on any 2xx
response. While hooks run the instance reports `service_ready: starting`;
when all succeeded it reports `ready`; if one still fails after the agent's
retries it reports `unhealthy`. The install wizard waits for `ready` before
it shows the app as usable.

Two rules:

- **Loopback only.** `url` must match
  `^http://(127\.0\.0\.1|localhost)(:[0-9]+)?/`. The agent refuses anything
  else, so a hook can only talk to the service it just started.
- **Idempotent.** Hooks run on every start and restart of the service, not
  just on install. Ollama's `/api/pull` of a model that is already present
  only verifies it, which is the kind of request that belongs here. A request
  that creates something new on each call does not.

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

For an app that serves its entry page under a **subpath** (e.g. MediaMTX at
`/cam/`), add `appPath` to the service. When the domain is bound at root, the
agent redirects a bare `/` to `appPath` so visitors land on the app; every
other path proxies through unchanged. Absolute path only. Additive — agents
that predate the field simply skip the redirect.

```yaml
services:
  - name: web
    targetType: host_port
    targetPort: 8888
    appPath: /cam/
```

For compose: `targetService` must reference a service in the body. Only
services listed here are reachable from the public domain — internal
databases/caches must not appear here.

While the upstream behind a bound domain does not answer yet (the container
is still running migrations, the host service is still pulling a model),
Caddy on the device serves a small "still starting" page that refreshes
itself every few seconds instead of a bare 502. This is agent behaviour and
needs no manifest field. Combine it with `readiness` below so the install
wizard also knows when the app is up.

## Readiness (v2.9)

"Installed" means the container or host service is running. Many apps need
more time than that before they answer: Open WebUI runs migrations and
pulls a model first, Nextcloud initialises its database. After the agent
reports an instance as installed it therefore probes the service with HTTP
GET requests, through the device's Caddy when a domain is bound and
directly otherwise, and reports the result as `service_ready`:

- `starting` until the first accepted response
- `ready` from the first accepted response on
- `unhealthy` after `timeoutSeconds` without one (the instance stays
  installed; the wizard tells the user the app is still starting)

Clients hold the install wizard's ready screen until `service_ready` is
`ready`. For `container` and `compose` runtimes the agent probes `/` with
the defaults even when the manifest says nothing. The `readiness` block on
a service entry only tunes that probe:

```yaml
services:
  - name: web
    targetType: container_port
    targetPort: 8080
    readiness:
      path: /health            # default "/"
      expectStatus: [200]      # default: any 200 to 399, plus 401 and 403
      timeoutSeconds: 600      # default 300, allowed 10 to 1800
```

- `path` must start with `/`, must not start with `//` and must not contain
  `://`.
- `expectStatus` lists the status codes that count as ready, each 100 to
  599. The default accepts 401 and 403 on purpose: an auth wall still means
  the app answers. Set it explicitly when the app's login page returns 200
  long before the app itself works, or when it only has a dedicated health
  endpoint.
- Raise `timeoutSeconds` for apps whose first start is slow. The probe
  stops at the timeout, so a value that is too low turns a healthy but slow
  install into `unhealthy`.

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

## Funding entry (mandatory in the official registry)

Every template in `obacht-registry` needs an entry in
`templates/funding.json`: which upstream project is behind it, whether that
project can receive money (`forwarded` / `no_channel` / `commercial` /
`declined` / `obacht`), and the payout channels. CI
(`validate-templates`) fails without one.

This drives the payment category: the per-template fee only applies where
the project has a payout channel ("no payout channel, no fee") —
`price_tier` in the catalog is derived from this status by
`scripts/sync-template-funding.js`, never edited by hand. The category is
a bucket, not an amount; billing maps it to amount + currency per
plan/region.

Verify channels against the project's `.github/FUNDING.yml` **and** the
GitHub Sponsors listing (FUNDING.yml entries can point at dead listings)
before adding them.

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
- [`examples/openwebui.yml`](../examples/openwebui.yml): single container consuming `openai_compatible`, with a `readiness` probe on `/health`
- [`examples/ollama-host.yml`](../examples/ollama-host.yml): macOS host service with `${host.data}` and a `post_start` model pull
