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

`obacht.dev/v2` is **stable** as of 2026-04. Current minor: **v2.1.0**
(2026-05) — adds compose-runtime bundles, compatibility hints, and the
`service_reference` config field type. v2.1 manifests are accepted by
agents on **v0.3.x** and newer; older agents reject them with a clear
"unsupported runtime/feature" error.

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
  minAgentVersion: "0.1.3"
  runtime:
    type: container               # or 'system' (systemd unit)
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
