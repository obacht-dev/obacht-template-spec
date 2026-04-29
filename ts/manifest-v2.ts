/**
 * TypeScript types + lightweight validator for the obacht.dev/v2 template
 * manifest. Mirrors schema/manifest-v2.json. Kept dependency-free so it can
 * be vendored into webapp/registry without pulling ajv/zod.
 *
 * For full schema validation in CI, prefer the JSON Schema directly with
 * ajv — these types are a developer ergonomics layer, not the authority.
 */

export const MANIFEST_V2_API_VERSION = 'obacht.dev/v2' as const;

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
  minAgentVersion?: string;
  exclusivityGroup?: string;
  runtime: ManifestV2Runtime;
  services?: ManifestV2Service[];
  configSchema?: ManifestV2ConfigField[];
  /**
   * S4.4: minimum host privilege the template needs.
   *  - 'none' (default): runs in the agent's docker-only sandbox.
   *  - 'power': needs the obacht-power sudoers entry; the agent
   *    refuses to install when system_settings.power_mode != 'enabled'.
   * Operators unlock Power Mode via `obachtctl system unlock-power`.
   */
  minSudoLevel?: 'none' | 'power';
  /**
   * S4.4: env-var keys whose values must be redacted from agent
   * telemetry, audit logs, and propagated error messages. The agent
   * applies redaction at the boundary of every emitted record.
   */
  secrets?: string[];
}

export type ManifestV2Runtime =
  | { type: 'container'; container: ManifestV2Container }
  | { type: 'system';    system:    ManifestV2System };

export interface ManifestV2Container {
  image: string;
  cmd?: string[];
  env?: Record<string, string>;
  ports?: Array<{ host: number; container: number }>;
  volumes?: Array<{ source: string; target: string; readOnly?: boolean }>;
  network?: string;
  labels?: Record<string, string>;
}

export interface ManifestV2System {
  unitName: string;
  unitTemplate: string;
  files?: Array<{ path: string; mode?: string; content: string }>;
}

export interface ManifestV2Service {
  name: string;
  targetType: 'container_port' | 'host_port' | 'unix_socket';
  targetPort?: number;
  targetPath?: string;
}

export interface ManifestV2ConfigField {
  key: string;
  label: string;
  type: 'text' | 'number' | 'select' | 'boolean' | 'secret';
  required?: boolean;
  default?: unknown;
  description?: string;
  options?: Array<{ value: string; label: string }>;
}

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
const KEBAB_RE  = /^[a-z][a-z0-9-]*[a-z0-9]$/;

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
  if (sp.minAgentVersion && !SEMVER_RE.test(sp.minAgentVersion)) {
    errors.push({ path: 'spec.minAgentVersion', message: 'must be semver' });
  }
  validateRuntime(sp.runtime as unknown, errors);
  if (sp.services !== undefined) {
    if (!Array.isArray(sp.services)) errors.push({ path: 'spec.services', message: 'must be an array' });
    else sp.services.forEach((s, i) => validateService(s, `spec.services[${i}]`, errors));
  }
  if (sp.configSchema !== undefined) {
    if (!Array.isArray(sp.configSchema)) errors.push({ path: 'spec.configSchema', message: 'must be an array' });
    else sp.configSchema.forEach((f, i) => validateConfigField(f, `spec.configSchema[${i}]`, errors));
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
        if (typeof k !== 'string' || !/^[A-Z][A-Z0-9_]*$/.test(k)) {
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
    if (rt.container.ports !== undefined && !Array.isArray(rt.container.ports)) {
      errors.push({ path: 'spec.runtime.container.ports', message: 'must be an array' });
    }
  } else if (rt.type === 'system') {
    if (!rt.system || typeof rt.system !== 'object') {
      errors.push({ path: 'spec.runtime.system', message: 'is required when type=system' });
      return;
    }
    if (!rt.system.unitName)     errors.push({ path: 'spec.runtime.system.unitName', message: 'is required' });
    if (!rt.system.unitTemplate) errors.push({ path: 'spec.runtime.system.unitTemplate', message: 'is required' });
  } else {
    errors.push({ path: 'spec.runtime.type', message: 'must be "container" or "system"' });
  }
}

function validateService(svc: any, path: string, errors: ValidationError[]): void {
  if (!svc || typeof svc !== 'object') {
    errors.push({ path, message: 'must be an object' });
    return;
  }
  if (!svc.name) errors.push({ path: `${path}.name`, message: 'is required' });
  const validTargets = ['container_port', 'host_port', 'unix_socket'];
  if (!validTargets.includes(svc.targetType)) {
    errors.push({ path: `${path}.targetType`, message: `must be one of ${validTargets.join(', ')}` });
  }
  if ((svc.targetType === 'container_port' || svc.targetType === 'host_port') && typeof svc.targetPort !== 'number') {
    errors.push({ path: `${path}.targetPort`, message: 'is required for *_port targets' });
  }
  if (svc.targetType === 'unix_socket' && !svc.targetPath) {
    errors.push({ path: `${path}.targetPath`, message: 'is required for unix_socket' });
  }
}

function validateConfigField(f: any, path: string, errors: ValidationError[]): void {
  if (!f || typeof f !== 'object') {
    errors.push({ path, message: 'must be an object' });
    return;
  }
  if (!f.key)   errors.push({ path: `${path}.key`,   message: 'is required' });
  if (!f.label) errors.push({ path: `${path}.label`, message: 'is required' });
  const validTypes = ['text', 'number', 'select', 'boolean', 'secret'];
  if (!validTypes.includes(f.type)) {
    errors.push({ path: `${path}.type`, message: `must be one of ${validTypes.join(', ')}` });
  }
  if (f.type === 'select' && (!Array.isArray(f.options) || f.options.length === 0)) {
    errors.push({ path: `${path}.options`, message: 'is required for type=select' });
  }
}
