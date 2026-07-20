# obacht-template-spec

Authoritative source of the **`obacht.dev/v2` template manifest** schema used
by the [obacht](https://obacht.dev) self-hosting platform.

A template manifest declares *what* should run on an obacht-managed device
(the workload, ingress contract, install-time configuration). The
[obacht-agent](https://github.com/obacht-dev/obacht-agent) on the device
turns this declaration into a running container — the manifest never
contains imperative install/uninstall steps or shell scripts.

This repo ships the spec in three forms so all three sides of the platform
share the exact same definition:

| Artefact                        | Consumer                        |
|---------------------------------|---------------------------------|
| [`schema/manifest-v2.json`](schema/manifest-v2.json) | JSON Schema (Draft-07) — used by CI / `obacht-registry` validation, IDE intellisense |
| [`ts/manifest-v2.ts`](ts/manifest-v2.ts)             | TypeScript types — consumed by `obacht-registry` and `obacht-webapp` |
| [`go/manifest/manifest.go`](go/manifest/manifest.go) | Go types — consumed by `obacht-agent` |

## Status

`obacht.dev/v2` is **stable** as of 2026-04. Current spec revision:
**v2.8**. v2.1 was the first revision to mandate `spec.minSpecVersion` and
`spec.compatibility` (breaking vs v2.0); **everything since v2.1 is
additive** — older agents simply ignore fields they don't understand, so a
manifest only needs to raise `minSpecVersion` if it depends on the agent
interpreting a newer feature (most newer features are client/registry-only).

Revisions since v2.1 (see [`docs/AUTHORING.md`](docs/AUTHORING.md) for detail):

| Rev  | Adds |
|------|------|
| v2.2 | `immutable` on config/secret fields (first-boot bootstrap values) |
| v2.3 | tag-only compose images (`allowUnpinnedImages`), `envConfigKey`, `${cfg.X}` structural placeholders |
| v2.4 | macOS platform — the `mac` device, `darwin/arm64`, `excludeDevices`, and the `system` runtime's `host_service` (launchd-managed host binary) |
| v2.5 | optional informational `spec.gettingStarted` post-install note |
| v2.6 | typed config-field renderers `timezone`, `email`, `domain` (render-only — value stays a plain string, so the agent/api are unaffected) |
| v2.7 | optional `advanced: true` on a config field — Easy-Mode clients hide it (install falls back to its `default`); render-only, agent/api unaffected |
| v2.8 | Pi system-runtime flavors `managed_service` (digest-pinned host binary as agent-generated hardened systemd unit) + `kiosk` marker; `compatibility.requiresFeatures`; `configField.optionsSource` (device-inventory selects). Withdraws the never-shipped free-form `unitName`/`unitTemplate` flavor |

## Quick reference

```yaml
apiVersion: obacht.dev/v2
kind: Template
metadata:
  name: whoami                    # kebab-case, unique template id
  displayName: whoami (echo)
  version: "1.0.0"                # semver
  trustLevel: official            # official | community | unverified
spec:
  minSpecVersion: "v2.1"          # mandatory since v2.1
  compatibility:                  # mandatory since v2.1
    architectures: ["linux/arm64", "linux/amd64"]
  runtime:
    type: container               # or 'compose' / 'system'
    container:
      image: traefik/whoami:latest
      cmd: ["--port=80"]
      env: { FOO: bar }
      ports: [{ host: 0, container: 80 }]
      volumes:
        - source: /var/lib/obacht/whoami/${instance.id}/data
          target: /data
      network: obacht-edge
  services:
    - name: web
      targetType: container_port
      targetPort: 80
  configSchema:
    - key: greeting
      label: Greeting text
      type: text
      default: "hi"
```

### Placeholder substitution

The agent (or the api when materializing the install) substitutes:

- `${instance.id}` — the per-install instance UUID
- `${cfg.<key>}` — value of the matching `configSchema` field

Substitution applies to `cmd`, `env` values, `volumes.source`, `volumes.target`,
`labels` values, and `network`. Anywhere else the manifest is taken verbatim.

### What v2 deliberately drops vs. v1

- per-template apt dependencies (templates only describe the workload)
- per-template install/uninstall shell scripts
- per-template DNS / cert handling (lives at device level via Caddy)
- per-template UI tabs that mutate device state directly

### v2.1 — compose runtime + compatibility

For multi-container "bundles" (e.g. a CMS + its database), set
`spec.runtime.type: compose` and provide:

```yaml
spec:
  runtime:
    type: compose
    compose:
      primaryService: web        # which service Caddy routes to
      primaryPort: 8080
      imageDigests:              # registry pins these at publish time
        ghost: sha256:a0506f3f05...
        mysql: sha256:7dcddc01f1...
      body: |
        services:
          web:
            image: ghost:5-alpine
            environment:
              database__client: mysql
              database__connection__host: db
          db:
            image: mysql:8.0
            environment:
              MYSQL_ROOT_PASSWORD: ${secret.db_root}
```

Each instance lands in its own compose project (`obacht-<instanceID>`)
with a private bundle network; only the `primaryService` is joined to
the shared `obacht-edge` so Caddy can reach it.

`spec.compatibility` lets a manifest declare what it needs from the
host so the install wizard can refuse incompatible devices up-front:

```yaml
compatibility:
  devices: ["raspberry-pi-4", "raspberry-pi-5"]
  architectures: ["linux/arm64"]
  resources:
    minRamMb: 1024
    minDiskMb: 2048
```

`configSchema[].type` gains `service_reference` — a free-text URL field
hinting that the value should point at another service (e.g. the URL
of an OpenWebUI bundle's Ollama backend).

### Config field types

`configSchema[].type` supports: `text`, `textarea`, `number`, `select`,
`boolean`, `secret`, `service_reference` (v2.1), and the v2.6 typed
renderers `timezone`, `email`, `domain`. The typed renderers are purely a
client concern — the webapp shows a better input widget (e.g. a searchable
IANA-timezone picker), but the value resolves to the same string a `text`
field would, so the agent and api need no change. Prefer them over a `text`
field with an "e.g. Europe/Berlin" hint.

Any field may also carry `immutable: true` (set-once at install) and, since
v2.7, `advanced: true` (hidden from the Easy-Mode UI — the install falls back
to the field's `default`). Both are render/policy flags; the agent ignores them.

## Layout

```
schema/manifest-v2.json     JSON Schema (the source of truth)
ts/manifest-v2.ts           TS types + validateManifestV2() helper
go/manifest/manifest.go     Go types
go/go.mod                   Go module: github.com/obacht-dev/obacht-template-spec/go
examples/                   Reference manifests (pulled from obacht-registry)
```

## Versioning

This repo is published per-release with git tags `vMAJOR.MINOR.PATCH`. The
`apiVersion` field inside the manifest evolves independently — bumping the
spec MAJOR signals consumer-breaking schema changes.

## License

MIT
