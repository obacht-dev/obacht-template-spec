/**
 * TypeScript types + lightweight validator for the obacht.dev/v2 template
 * manifest, spec revision v2.1. Mirrors schema/manifest-v2.json. Kept
 * dependency-free so it can be vendored into webapp/registry without
 * pulling ajv/zod.
 *
 * For full structural validation in CI, prefer the JSON Schema directly
 * with ajv — these types are a developer ergonomics layer, not the
 * authority. Compose-body allowlist validation lives in
 * compose-allowlist.ts.
 */

export const MANIFEST_V2_API_VERSION = 'obacht.dev/v2' as const;
// v2.4: adds the macOS platform — the `mac` device, the `darwin/arm64`
// architecture, and the `system` runtime's host-service flavor (launchd-managed
// host binary, e.g. Ollama). Additive over v2.3, so it stays a minor within the
// obacht.dev/v2 envelope.
// v2.5: adds the optional, informational spec.gettingStarted note (shown to the
// user post-install). Additive over v2.4 — old manifests stay valid.
// v2.6: adds the typed config-field renderers 'timezone', 'email' and 'domain'.
// These are render-only: clients (webapp) show a better input widget, but the
// value still resolves to the same string a 'text' field would, so the agent and
// api need no change. Additive over v2.5 — old manifests stay valid.
// v2.7: adds the optional `advanced` flag on config fields. Render-only: Easy-Mode
// clients hide flagged fields (the install falls back to the field default), the
// agent and api ignore it. Additive over v2.6 — old manifests stay valid.
// v2.8: adds the Pi system-runtime flavors `managed_service` (digest-pinned host
// binary run as an agent-generated hardened systemd unit; requires
// minSudoLevel: power) and `kiosk` (marker for the agent-shipped kiosk session),
// `compatibility.requiresFeatures` (device-feature gating, e.g. a preinstalled
// desktop), and `configField.optionsSource` (render-only: select options
// populated from device-reported inventory such as detected cameras). v2.8 also
// WITHDRAWS the never-shipped free-form systemd flavor (unitName +
// unitTemplate): system templates never author unit text — registry publish and
// the agent materializer both reject it. No shipped manifest is affected
// (verified: zero users of the flavor).
export const SUPPORTED_SPEC_VERSION = 'v2.8' as const;

export interface ManifestV2 {
  apiVersion: typeof MANIFEST_V2_API_VERSION;
  kind: 'Template';
  metadata: ManifestV2Metadata;
  spec: ManifestV2Spec;
}

export interface ManifestV2Metadata {
  name: string;
  displayName: string;
  description?: string;
  version: string;
  author?: string;
  license?: string;
  homepage?: string;
  icon?: string;
  tags?: string[];
  trustLevel?: 'official' | 'community' | 'unverified';
}

export interface ManifestV2Spec {
  /** Spec revision the manifest depends on, e.g. "v2.1". */
  minSpecVersion: string;
  minAgentVersion?: string;
  exclusivityGroup?: string;
  compatibility: ManifestV2Compatibility;
  runtime: ManifestV2Runtime;
  services?: ManifestV2Service[];
  configSchema?: ManifestV2ConfigField[];
  secretsSchema?: ManifestV2SecretField[];
  provides?: ManifestV2ProvideEntry[];
  consumes?: ManifestV2ConsumeEntry[];
  minSudoLevel?: 'none' | 'power';
  /** Env-var keys whose values are redacted in agent telemetry/audit/errors. */
  secrets?: string[];
  /**
   * Optional post-install "getting started" note shown to the user after the
   * service is installed (e.g. "open the app and register your first
   * account"). Plain text / lightweight markdown. Purely informational — the
   * agent never sees it.
   */
  gettingStarted?: string;
}

export type DeviceModel =
  | 'raspberry-pi-4'
  | 'raspberry-pi-5'
  | 'mac-mini-arm'
  | 'mac'
  | 'generic-x86_64'
  | 'generic-arm64';

export type Architecture = 'linux/arm64' | 'linux/amd64' | 'linux/arm/v7' | 'darwin/arm64';

// v2.8: device features a template can require. The agent detects and reports
// these; api install-plan compat check, client catalog filter AND the on-device
// install assertion all enforce requiresFeatures ⊆ reported features. Closed
// enum — widening it is a deliberate spec bump.
export type DeviceFeature = 'desktop-chromium' | 'wayland-compositor' | 'csi-or-usb-camera';

export interface ManifestV2Compatibility {
  devices?: DeviceModel[];
  // v2.4: device classes the template should NOT be offered on even if the
  // architecture matches — e.g. a Pi container bundle that has a native Mac
  // replacement (so it's hidden on `mac`). Clients filter on this.
  excludeDevices?: DeviceModel[];
  architectures: Architecture[];
  // v2.8: see DeviceFeature.
  requiresFeatures?: DeviceFeature[];
  os?: Array<{ id: string; minVersion?: string }>;
  resources?: { minRamMb?: number; minDiskMb?: number };
}

export type ManifestV2Runtime =
  | { type: 'container'; container: ManifestV2Container }
  | { type: 'compose';   compose:   ManifestV2Compose }
  | { type: 'system';    system:    ManifestV2System };

export interface ManifestV2Container {
  image: string;
  imageDigest?: string;
  cmd?: string[];
  env?: Record<string, string>;
  ports?: Array<{ host: number; container: number }>;
  volumes?: Array<{ source: string; target: string; readOnly?: boolean }>;
  network?: string;
  labels?: Record<string, string>;
}

export interface ManifestV2Compose {
  primaryService: string;
  /** Number, or a ${cfg.X} placeholder for custom-docker-composition. */
  primaryPort: number | string;
  dataPath?: string;
  /** Map from image reference (as in body) to sha256 digest. Populated by registry publish. */
  imageDigests?: Record<string, string>;
  /** YAML compose document (string). Validated against allowlist by registry. */
  body: string;
  /** v2.3: allow tag-only images (custom-docker-composition); agent enforces allowlist at apply. */
  allowUnpinnedImages?: boolean;
  /** v2.3: config key written verbatim as the project .env file. */
  envConfigKey?: string;
}

export interface ManifestV2System {
  // Exactly one flavor must be present. The pre-v2.8 free-form systemd flavor
  // (unitName + unitTemplate) is withdrawn — system templates never author
  // unit text; registry publish and the agent materializer reject it.
  //
  // v2.4 (macOS host-services): a launchd-managed host binary instead of a
  // systemd unit, e.g. Ollama. Structured (binary + argv + env), never a raw
  // plist/shell. The agent verifies binary_digest before extract/exec.
  host_service?: ManifestV2HostService;
  // v2.8 (Raspberry Pi / Linux, requires minSudoLevel: power): a digest-pinned
  // host binary run as a hardened systemd unit that the AGENT generates
  // (DynamicUser, DevicePolicy=closed + declared DeviceAllow, NoNewPrivileges,
  // ProtectSystem=strict); the root helper independently re-validates the unit
  // before installing it.
  managed_service?: ManifestV2ManagedService;
  // v2.8 marker flavor (requires minSudoLevel: power and
  // compatibility.requiresFeatures [desktop-chromium, wayland-compositor]):
  // the agent-shipped kiosk session. All privileged behaviour lives in the
  // agent; the template contributes configSchema + files only.
  kiosk?: Record<string, never>;
  files?: Array<{ path: string; mode?: string; content: string }>;
}

export interface ManifestV2HostService {
  kind?: string;
  binary: string;
  binary_url: string;
  binary_digest: string;
  archive?: 'tgz';
  args?: string[];
  env?: Record<string, string>;
  data_dir?: string;
}

// v2.8: hardware access grants for managed_service. Closed enums — widening
// them is a deliberate spec bump.
export type ManagedServiceGroup = 'video' | 'render' | 'input';
export type ManagedServiceDevicePattern = '/dev/video*' | '/dev/media*' | '/dev/dri/*';

export interface ManifestV2ManagedService {
  kind?: string;
  /** Must be on the agent's closed binary allowlist (e.g. 'mediamtx'). */
  binary: string;
  /** https only; host must be on the agent's download-host allowlist. */
  binary_url: string;
  binary_digest: string;
  archive?: 'tgz';
  args?: string[];
  env?: Record<string, string>;
  hardware?: {
    groups?: ManagedServiceGroup[];
    devices?: ManagedServiceDevicePattern[];
  };
  /**
   * Local ports the service binds (documentation + validation only; exposure
   * happens exclusively via spec.services → device Caddy).
   */
  listen_ports?: number[];
}

export interface ManifestV2Service {
  name: string;
  targetType: 'container_port' | 'host_port' | 'unix_socket';
  /** Required when runtime.type === 'compose'. Must reference a service in the compose body. */
  targetService?: string;
  targetPort?: number;
  targetPath?: string;
}

export type ManifestV2ConfigType =
  | 'text'
  | 'textarea'
  | 'number'
  | 'select'
  | 'boolean'
  | 'secret'
  | 'service_reference'
  // v2.6 render-only typed inputs. Value is still a plain string.
  | 'timezone'
  | 'email'
  | 'domain';

export interface ManifestV2ConfigField {
  key: string;
  label: string;
  type: ManifestV2ConfigType;
  required?: boolean;
  default?: unknown;
  description?: string;
  options?: Array<{ value: string; label: string }>;
  /**
   * Spec v2.8 (render-only, select fields, mutually exclusive with options):
   * populate the select options from device-reported inventory instead of a
   * static list. Clients must block the install while the inventory is empty
   * or the device is offline — no silent free-text fallback. The agent and
   * api treat the chosen value as a plain string, exactly like a static
   * select.
   */
  optionsSource?: { kind: 'device_inventory'; inventory: 'cameras' };
  /** For type=service_reference. */
  interface?: string;
  interfaceVersion?: string;
  fallback?: { type: 'text' | 'secret'; placeholder?: string; default?: string };
  /**
   * Spec v2.2: when true, the value is set at install time and cannot be
   * changed afterwards. The webapp disables the field in the post-install
   * Configure dialog and the api rejects install-plan submissions that try
   * to mutate it. Use for values the underlying app only honours during
   * first-boot bootstrap.
   */
  immutable?: boolean;
  /**
   * Spec v2.7: when true, Easy-Mode clients hide this field from the
   * install/Configure UI; the install falls back to the field's `default`.
   * Render-only — the agent and api ignore it. Only flag fields that have a
   * usable default and are not required-without-default, otherwise an
   * Easy-Mode install would be left with no value.
   */
  advanced?: boolean;
}

export interface ManifestV2SecretField {
  key: string;
  length: number;
  charset?: 'alphanumeric' | 'alphanumeric_symbols' | 'hex' | 'base64' | 'base64_bytes';
  /**
   * Spec v2.2: secrets are immutable by default once generated; this flag
   * is accepted for explicit declaration. Setting it to false is currently
   * not honoured by the agent (no rotation path).
   */
  immutable?: boolean;
}

export interface ManifestV2ProvideEntry {
  interface: string;
  version: string;
  service: string;
  port: number;
  path?: string;
  auth?: 'none' | 'bearer_token' | 'basic';
}

export interface ManifestV2ConsumeEntry {
  interface: string;
  version: string;
  configKey: string;
}

// ---------------------------------------------------------------------------
// Reserved interface names (see PLAN-MANIFEST-V2.1 §1.8)
// ---------------------------------------------------------------------------

export const RESERVED_INTERFACES = [
  'openai_compatible',
  's3_compatible',
  'smtp_relay',
  'postgres_database',
  'oidc_provider',
] as const;

// ---------------------------------------------------------------------------
// Lightweight runtime validator
// ---------------------------------------------------------------------------

export interface ValidationError {
  path: string;
  message: string;
}

export interface ValidationResult {
  valid: boolean;
  errors: ValidationError[];
}

const SEMVER_RE = /^\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$/;
const KEBAB_RE = /^[a-z][a-z0-9-]*[a-z0-9]$/;
const SPEC_VERSION_RE = /^v\d+\.\d+$/;
const IFACE_RE = /^[a-z][a-z0-9_]*$/;
const IFACE_VERSION_RE = /^v\d+$/;
const ENV_KEY_RE = /^[A-Z][A-Z0-9_]*$/;
const SECRET_KEY_RE = /^[a-z][a-z0-9_]*$/;
const CFG_KEY_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/;
const DIGEST_RE = /^sha256:[a-f0-9]{64}$/;
// A bare ${cfg.X} placeholder, used by custom-docker-composition to drive
// structural fields (primaryPort, service targetPort/targetService) from
// user config resolved on the device at install time.
const CFG_REF_RE = /^\$\{cfg\.[a-zA-Z0-9_]+\}$/;

const VALID_DEVICES: DeviceModel[] = [
  'raspberry-pi-4',
  'raspberry-pi-5',
  'mac-mini-arm',
  'mac',
  'generic-x86_64',
  'generic-arm64',
];
const VALID_ARCHS: Architecture[] = ['linux/arm64', 'linux/amd64', 'linux/arm/v7', 'darwin/arm64'];
const VALID_FEATURES: DeviceFeature[] = ['desktop-chromium', 'wayland-compositor', 'csi-or-usb-camera'];
const VALID_MANAGED_GROUPS: ManagedServiceGroup[] = ['video', 'render', 'input'];
const VALID_MANAGED_DEVICES: ManagedServiceDevicePattern[] = ['/dev/video*', '/dev/media*', '/dev/dri/*'];
const VALID_INVENTORIES = ['cameras'];
const MANAGED_BINARY_RE = /^[a-z][a-z0-9-]*$/;

export function validateManifestV2(input: unknown): ValidationResult {
  const errors: ValidationError[] = [];
  const m = input as Partial<ManifestV2>;
  if (!m || typeof m !== 'object') {
    return { valid: false, errors: [{ path: '', message: 'manifest must be an object' }] };
  }
  if (m.apiVersion !== MANIFEST_V2_API_VERSION) {
    errors.push({ path: 'apiVersion', message: `must be "${MANIFEST_V2_API_VERSION}"` });
  }
  if (m.kind !== 'Template') {
    errors.push({ path: 'kind', message: 'must be "Template"' });
  }
  validateMetadata(m.metadata, errors);
  validateSpec(m.spec, errors);
  return { valid: errors.length === 0, errors };
}

function validateMetadata(md: ManifestV2Metadata | undefined, errors: ValidationError[]): void {
  if (!md || typeof md !== 'object') {
    errors.push({ path: 'metadata', message: 'is required' });
    return;
  }
  if (!md.name || !KEBAB_RE.test(md.name)) errors.push({ path: 'metadata.name', message: 'must be kebab-case' });
  if (!md.displayName) errors.push({ path: 'metadata.displayName', message: 'is required' });
  if (!md.version || !SEMVER_RE.test(md.version)) errors.push({ path: 'metadata.version', message: 'must be semver' });
  if (md.trustLevel && !['official', 'community', 'unverified'].includes(md.trustLevel)) {
    errors.push({ path: 'metadata.trustLevel', message: 'invalid' });
  }
}

function validateSpec(sp: ManifestV2Spec | undefined, errors: ValidationError[]): void {
  if (!sp || typeof sp !== 'object') {
    errors.push({ path: 'spec', message: 'is required' });
    return;
  }
  if (!sp.minSpecVersion || !SPEC_VERSION_RE.test(sp.minSpecVersion)) {
    errors.push({ path: 'spec.minSpecVersion', message: 'is required and must match v<major>.<minor>' });
  }
  if (sp.minAgentVersion && !SEMVER_RE.test(sp.minAgentVersion)) {
    errors.push({ path: 'spec.minAgentVersion', message: 'must be semver' });
  }
  validateCompatibility(sp.compatibility, errors);
  validateRuntime(sp.runtime as unknown, errors);

  // Cross-validation: compose runtime requires services[].targetService
  const runtimeType = (sp.runtime as { type?: string } | undefined)?.type;
  if (sp.services !== undefined) {
    if (!Array.isArray(sp.services)) errors.push({ path: 'spec.services', message: 'must be an array' });
    else sp.services.forEach((s, i) => validateService(s, `spec.services[${i}]`, errors, runtimeType));
  }

  if (sp.configSchema !== undefined) {
    if (!Array.isArray(sp.configSchema)) errors.push({ path: 'spec.configSchema', message: 'must be an array' });
    else sp.configSchema.forEach((f, i) => validateConfigField(f, `spec.configSchema[${i}]`, errors));
  }

  if (sp.secretsSchema !== undefined) {
    if (!Array.isArray(sp.secretsSchema)) errors.push({ path: 'spec.secretsSchema', message: 'must be an array' });
    else {
      const seen = new Set<string>();
      sp.secretsSchema.forEach((f, i) => {
        const path = `spec.secretsSchema[${i}]`;
        if (!f.key || !SECRET_KEY_RE.test(f.key)) errors.push({ path: `${path}.key`, message: 'must be lowercase snake_case' });
        else if (seen.has(f.key)) errors.push({ path: `${path}.key`, message: `duplicate key '${f.key}'` });
        else seen.add(f.key);
        if (typeof f.length !== 'number' || f.length < 8 || f.length > 256) {
          errors.push({ path: `${path}.length`, message: 'must be 8..256' });
        }
        if (f.charset && !['alphanumeric', 'alphanumeric_symbols', 'hex', 'base64', 'base64_bytes'].includes(f.charset)) {
          errors.push({ path: `${path}.charset`, message: 'invalid' });
        }
      });
    }
  }

  if (sp.provides !== undefined) {
    if (!Array.isArray(sp.provides)) errors.push({ path: 'spec.provides', message: 'must be an array' });
    else sp.provides.forEach((p, i) => validateProvide(p, `spec.provides[${i}]`, errors));
  }

  if (sp.consumes !== undefined) {
    if (!Array.isArray(sp.consumes)) errors.push({ path: 'spec.consumes', message: 'must be an array' });
    else sp.consumes.forEach((c, i) => validateConsume(c, `spec.consumes[${i}]`, errors, sp.configSchema));
  }

  if (sp.minSudoLevel !== undefined && sp.minSudoLevel !== 'none' && sp.minSudoLevel !== 'power') {
    errors.push({ path: 'spec.minSudoLevel', message: "must be 'none' or 'power'" });
  }
  if (sp.secrets !== undefined) {
    if (!Array.isArray(sp.secrets)) {
      errors.push({ path: 'spec.secrets', message: 'must be an array of env keys' });
    } else {
      const seen = new Set<string>();
      sp.secrets.forEach((k, i) => {
        if (typeof k !== 'string' || !ENV_KEY_RE.test(k)) {
          errors.push({ path: `spec.secrets[${i}]`, message: 'must be SHELL_ENV_KEY (uppercase + underscore)' });
        } else if (seen.has(k)) {
          errors.push({ path: `spec.secrets[${i}]`, message: `duplicate key '${k}'` });
        } else {
          seen.add(k);
        }
      });
    }
  }
}

function validateCompatibility(c: ManifestV2Compatibility | undefined, errors: ValidationError[]): void {
  if (!c || typeof c !== 'object') {
    errors.push({ path: 'spec.compatibility', message: 'is required' });
    return;
  }
  if (!Array.isArray(c.architectures) || c.architectures.length === 0) {
    errors.push({ path: 'spec.compatibility.architectures', message: 'must be a non-empty array' });
  } else {
    c.architectures.forEach((a, i) => {
      if (!VALID_ARCHS.includes(a)) errors.push({ path: `spec.compatibility.architectures[${i}]`, message: `invalid architecture '${a}'` });
    });
  }
  if (c.devices !== undefined) {
    if (!Array.isArray(c.devices)) errors.push({ path: 'spec.compatibility.devices', message: 'must be an array' });
    else c.devices.forEach((d, i) => {
      if (!VALID_DEVICES.includes(d)) errors.push({ path: `spec.compatibility.devices[${i}]`, message: `invalid device '${d}'` });
    });
  }
  if (c.excludeDevices !== undefined) {
    if (!Array.isArray(c.excludeDevices)) errors.push({ path: 'spec.compatibility.excludeDevices', message: 'must be an array' });
    else c.excludeDevices.forEach((d, i) => {
      if (!VALID_DEVICES.includes(d)) errors.push({ path: `spec.compatibility.excludeDevices[${i}]`, message: `invalid device '${d}'` });
    });
  }
  if (c.requiresFeatures !== undefined) {
    if (!Array.isArray(c.requiresFeatures)) errors.push({ path: 'spec.compatibility.requiresFeatures', message: 'must be an array' });
    else c.requiresFeatures.forEach((f, i) => {
      if (!VALID_FEATURES.includes(f)) errors.push({ path: `spec.compatibility.requiresFeatures[${i}]`, message: `invalid feature '${f}'` });
    });
  }
  if (c.resources) {
    if (c.resources.minRamMb !== undefined && (!Number.isInteger(c.resources.minRamMb) || c.resources.minRamMb < 32)) {
      errors.push({ path: 'spec.compatibility.resources.minRamMb', message: 'must be an integer >= 32' });
    }
    if (c.resources.minDiskMb !== undefined && (!Number.isInteger(c.resources.minDiskMb) || c.resources.minDiskMb < 32)) {
      errors.push({ path: 'spec.compatibility.resources.minDiskMb', message: 'must be an integer >= 32' });
    }
  }
}

function validateRuntime(rt: any, errors: ValidationError[]): void {
  if (!rt || typeof rt !== 'object') {
    errors.push({ path: 'spec.runtime', message: 'is required' });
    return;
  }
  if (rt.type === 'container') {
    if (!rt.container || typeof rt.container !== 'object') {
      errors.push({ path: 'spec.runtime.container', message: 'is required when type=container' });
      return;
    }
    if (!rt.container.image) errors.push({ path: 'spec.runtime.container.image', message: 'is required' });
    if (rt.container.imageDigest && !DIGEST_RE.test(rt.container.imageDigest)) {
      errors.push({ path: 'spec.runtime.container.imageDigest', message: 'must be sha256:<hex64>' });
    }
    if (rt.container.ports !== undefined && !Array.isArray(rt.container.ports)) {
      errors.push({ path: 'spec.runtime.container.ports', message: 'must be an array' });
    }
  } else if (rt.type === 'compose') {
    if (!rt.compose || typeof rt.compose !== 'object') {
      errors.push({ path: 'spec.runtime.compose', message: 'is required when type=compose' });
      return;
    }
    if (!rt.compose.primaryService) errors.push({ path: 'spec.runtime.compose.primaryService', message: 'is required' });
    {
      const pp = rt.compose.primaryPort;
      const ok = (typeof pp === 'number' && pp >= 1 && pp <= 65535) || (typeof pp === 'string' && CFG_REF_RE.test(pp));
      if (!ok) errors.push({ path: 'spec.runtime.compose.primaryPort', message: 'must be 1..65535 or a ${cfg.X} placeholder' });
    }
    if (typeof rt.compose.body !== 'string' || rt.compose.body.trim().length === 0) {
      errors.push({ path: 'spec.runtime.compose.body', message: 'is required (YAML compose document as string)' });
    }
    if (rt.compose.imageDigests) {
      if (typeof rt.compose.imageDigests !== 'object') {
        errors.push({ path: 'spec.runtime.compose.imageDigests', message: 'must be an object' });
      } else {
        for (const [ref, digest] of Object.entries(rt.compose.imageDigests)) {
          if (typeof digest !== 'string' || !DIGEST_RE.test(digest)) {
            errors.push({ path: `spec.runtime.compose.imageDigests["${ref}"]`, message: 'must be sha256:<hex64>' });
          }
        }
      }
    }
  } else if (rt.type === 'system') {
    if (!rt.system || typeof rt.system !== 'object') {
      errors.push({ path: 'spec.runtime.system', message: 'is required when type=system' });
      return;
    }
    // v2.8: the free-form systemd flavor is withdrawn — reject explicitly.
    if (rt.system.unitName !== undefined || rt.system.unitTemplate !== undefined) {
      errors.push({
        path: 'spec.runtime.system',
        message: 'the free-form systemd flavor (unitName/unitTemplate) was withdrawn in v2.8 — use managed_service (the agent generates the unit)',
      });
    }
    const flavors = ['host_service', 'managed_service', 'kiosk'].filter((k) => rt.system[k] !== undefined);
    if (flavors.length !== 1) {
      errors.push({ path: 'spec.runtime.system', message: 'exactly one flavor of host_service, managed_service or kiosk is required' });
    }
    if (rt.system.host_service) {
      // v2.4 macOS host-service flavor: structured binary + pinned digest.
      const hs = rt.system.host_service;
      if (!hs.binary) errors.push({ path: 'spec.runtime.system.host_service.binary', message: 'is required' });
      if (!hs.binary_url) errors.push({ path: 'spec.runtime.system.host_service.binary_url', message: 'is required' });
      if (!hs.binary_digest || !DIGEST_RE.test(hs.binary_digest)) {
        errors.push({ path: 'spec.runtime.system.host_service.binary_digest', message: 'is required and must be sha256:<hex64>' });
      }
      if (hs.archive !== undefined && hs.archive !== 'tgz') {
        errors.push({ path: 'spec.runtime.system.host_service.archive', message: 'must be "tgz"' });
      }
    }
    if (rt.system.managed_service) {
      // v2.8 Linux managed-service flavor: pinned binary + closed hardware enums.
      const ms = rt.system.managed_service;
      const p = 'spec.runtime.system.managed_service';
      if (!ms.binary || !MANAGED_BINARY_RE.test(ms.binary)) {
        errors.push({ path: `${p}.binary`, message: 'is required (lowercase kebab, e.g. "mediamtx")' });
      }
      if (!ms.binary_url || typeof ms.binary_url !== 'string' || !ms.binary_url.startsWith('https://')) {
        errors.push({ path: `${p}.binary_url`, message: 'is required and must be https://' });
      }
      if (!ms.binary_digest || !DIGEST_RE.test(ms.binary_digest)) {
        errors.push({ path: `${p}.binary_digest`, message: 'is required and must be sha256:<hex64>' });
      }
      if (ms.archive !== undefined && ms.archive !== 'tgz') {
        errors.push({ path: `${p}.archive`, message: 'must be "tgz"' });
      }
      if (ms.hardware !== undefined) {
        if (typeof ms.hardware !== 'object' || ms.hardware === null) {
          errors.push({ path: `${p}.hardware`, message: 'must be an object' });
        } else {
          (ms.hardware.groups ?? []).forEach((g: string, i: number) => {
            if (!VALID_MANAGED_GROUPS.includes(g as ManagedServiceGroup)) {
              errors.push({ path: `${p}.hardware.groups[${i}]`, message: `invalid group '${g}' (allowed: ${VALID_MANAGED_GROUPS.join(', ')})` });
            }
          });
          (ms.hardware.devices ?? []).forEach((d: string, i: number) => {
            if (!VALID_MANAGED_DEVICES.includes(d as ManagedServiceDevicePattern)) {
              errors.push({ path: `${p}.hardware.devices[${i}]`, message: `invalid device pattern '${d}' (allowed: ${VALID_MANAGED_DEVICES.join(', ')})` });
            }
          });
        }
      }
      if (ms.listen_ports !== undefined) {
        if (!Array.isArray(ms.listen_ports)) errors.push({ path: `${p}.listen_ports`, message: 'must be an array' });
        else ms.listen_ports.forEach((lp: unknown, i: number) => {
          if (typeof lp !== 'number' || !Number.isInteger(lp) || lp < 1 || lp > 65535) {
            errors.push({ path: `${p}.listen_ports[${i}]`, message: 'must be 1..65535' });
          }
        });
      }
    }
    if (rt.system.kiosk !== undefined) {
      // v2.8 marker flavor: must be an empty object — all behaviour is agent-shipped.
      if (typeof rt.system.kiosk !== 'object' || rt.system.kiosk === null || Object.keys(rt.system.kiosk).length !== 0) {
        errors.push({ path: 'spec.runtime.system.kiosk', message: 'must be an empty object (marker flavor — behaviour lives in the agent)' });
      }
    }
  } else {
    errors.push({ path: 'spec.runtime.type', message: 'must be "container", "compose" or "system"' });
  }
}

function validateService(svc: any, path: string, errors: ValidationError[], runtimeType: string | undefined): void {
  if (!svc || typeof svc !== 'object') {
    errors.push({ path, message: 'must be an object' });
    return;
  }
  if (!svc.name) errors.push({ path: `${path}.name`, message: 'is required' });
  const validTargets = ['container_port', 'host_port', 'unix_socket'];
  if (!validTargets.includes(svc.targetType)) {
    errors.push({ path: `${path}.targetType`, message: `must be one of ${validTargets.join(', ')}` });
  }
  if ((svc.targetType === 'container_port' || svc.targetType === 'host_port') &&
      !(typeof svc.targetPort === 'number' || (typeof svc.targetPort === 'string' && CFG_REF_RE.test(svc.targetPort)))) {
    errors.push({ path: `${path}.targetPort`, message: 'is required for *_port targets (number or ${cfg.X})' });
  }
  if (svc.targetType === 'unix_socket' && !svc.targetPath) {
    errors.push({ path: `${path}.targetPath`, message: 'is required for unix_socket' });
  }
  if (runtimeType === 'compose' && (!svc.targetService || typeof svc.targetService !== 'string')) {
    errors.push({ path: `${path}.targetService`, message: 'is required when runtime.type=compose' });
  }
}

function validateConfigField(f: any, path: string, errors: ValidationError[]): void {
  if (!f || typeof f !== 'object') {
    errors.push({ path, message: 'must be an object' });
    return;
  }
  if (!f.key || !CFG_KEY_RE.test(f.key)) errors.push({ path: `${path}.key`, message: 'is required (alphanumeric + underscore, must start with letter/underscore)' });
  if (!f.label) errors.push({ path: `${path}.label`, message: 'is required' });
  const validTypes = ['text', 'textarea', 'number', 'select', 'boolean', 'secret', 'service_reference', 'timezone', 'email', 'domain'];
  if (!validTypes.includes(f.type)) {
    errors.push({ path: `${path}.type`, message: `must be one of ${validTypes.join(', ')}` });
  }
  if (f.options !== undefined && f.optionsSource !== undefined) {
    errors.push({ path: `${path}.optionsSource`, message: 'options and optionsSource are mutually exclusive' });
  }
  if (f.optionsSource !== undefined) {
    if (typeof f.optionsSource !== 'object' || f.optionsSource === null) {
      errors.push({ path: `${path}.optionsSource`, message: 'must be an object' });
    } else {
      if (f.optionsSource.kind !== 'device_inventory') {
        errors.push({ path: `${path}.optionsSource.kind`, message: 'must be "device_inventory"' });
      }
      if (!VALID_INVENTORIES.includes(f.optionsSource.inventory)) {
        errors.push({ path: `${path}.optionsSource.inventory`, message: `must be one of ${VALID_INVENTORIES.join(', ')}` });
      }
      if (f.type !== 'select') {
        errors.push({ path: `${path}.optionsSource`, message: 'is only allowed on type=select fields' });
      }
    }
  }
  if (f.type === 'select' && f.optionsSource === undefined && (!Array.isArray(f.options) || f.options.length === 0)) {
    errors.push({ path: `${path}.options`, message: 'is required for type=select (or use optionsSource)' });
  }
  if (f.type === 'service_reference') {
    if (!f.interface || !IFACE_RE.test(f.interface)) errors.push({ path: `${path}.interface`, message: 'is required for type=service_reference' });
    if (!f.interfaceVersion || !IFACE_VERSION_RE.test(f.interfaceVersion)) errors.push({ path: `${path}.interfaceVersion`, message: 'is required and must be v<major>' });
  }
}

function validateProvide(p: any, path: string, errors: ValidationError[]): void {
  if (!p || typeof p !== 'object') { errors.push({ path, message: 'must be an object' }); return; }
  if (!p.interface || !IFACE_RE.test(p.interface)) errors.push({ path: `${path}.interface`, message: 'must be lowercase snake_case' });
  if (!p.version || !IFACE_VERSION_RE.test(p.version)) errors.push({ path: `${path}.version`, message: 'must be v<major>' });
  if (!p.service) errors.push({ path: `${path}.service`, message: 'is required' });
  if (typeof p.port !== 'number' || p.port < 1 || p.port > 65535) errors.push({ path: `${path}.port`, message: 'must be 1..65535' });
}

function validateConsume(c: any, path: string, errors: ValidationError[], schema?: ManifestV2ConfigField[]): void {
  if (!c || typeof c !== 'object') { errors.push({ path, message: 'must be an object' }); return; }
  if (!c.interface || !IFACE_RE.test(c.interface)) errors.push({ path: `${path}.interface`, message: 'must be lowercase snake_case' });
  if (!c.version || !IFACE_VERSION_RE.test(c.version)) errors.push({ path: `${path}.version`, message: 'must be v<major>' });
  if (!c.configKey) errors.push({ path: `${path}.configKey`, message: 'is required' });
  // Cross-check that configKey points to a service_reference in configSchema
  if (c.configKey && schema) {
    const cf = schema.find((f) => f.key === c.configKey);
    if (!cf) errors.push({ path: `${path}.configKey`, message: `references unknown configSchema key '${c.configKey}'` });
    else if (cf.type !== 'service_reference') errors.push({ path: `${path}.configKey`, message: `configSchema['${c.configKey}'] must be of type 'service_reference'` });
  }
}
