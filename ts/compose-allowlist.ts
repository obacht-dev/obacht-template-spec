/**
 * Compose-body allowlist validator (spec v2.1).
 *
 * Parses the YAML compose body string from spec.runtime.compose.body and
 * walks every top-level + per-service key against an obacht-specific
 * allowlist. Rejects on the first forbidden key with a precise error.
 *
 * Used by the registry validator at publish time AND by the agent as
 * defence-in-depth before docker compose up. Both layers should refuse
 * the same things.
 */

import * as yaml from 'js-yaml';

export interface ComposeAllowlistError {
  path: string;
  message: string;
}

export interface ComposeAllowlistResult {
  valid: boolean;
  errors: ComposeAllowlistError[];
  /** Distinct image references (as written in body, without digest pinning). */
  images: string[];
  /** Service names declared in the body. */
  serviceNames: string[];
  /** Named volumes declared at the top level. */
  volumeNames: string[];
}

const ALLOWED_TOP_LEVEL = new Set(['services', 'volumes', 'networks', 'version']);

const ALLOWED_SERVICE_KEYS = new Set([
  'image',
  'command',
  'entrypoint',
  'environment',
  'volumes',
  'depends_on',
  'healthcheck',
  'restart',
  'labels',
  'networks',
  'tmpfs',
  'read_only',
  'user',
  'working_dir',
  'cap_drop',
  'security_opt',
  'stop_grace_period',
  'stop_signal',
  'sysctls',
  'shm_size',
  'mem_limit',
  'cpus',
  'init',
]);

const FORBIDDEN_SERVICE_KEYS_REASONS: Record<string, string> = {
  build:        'templates ship images, never build on the Pi',
  network_mode: 'bundle isolation requires obacht-managed networks',
  privileged:   'defeats sandboxing',
  cap_add:      'closed list — use system runtime + Power Mode if you need privileges',
  devices:      'closed list — use system runtime + Power Mode if you need device access',
  pid:          'namespace bypass not allowed',
  ipc:          'namespace bypass not allowed',
  uts:          'namespace bypass not allowed',
  ports:        'host port mapping is owned by the device Caddy via spec.services',
  expose:       'use spec.services to expose ports',
  secrets:      "use obacht's ${secret.x} substitution",
  configs:      "use obacht's ${cfg.x} substitution",
  extends:      'no external compose files allowed',
  profiles:     'all declared services must always run',
  external_links: 'cross-bundle wiring goes through Interfaces',
  links:        'use depends_on instead',
};

const ALLOWED_RESTART = new Set(['unless-stopped', 'on-failure', 'no']);
const ALLOWED_SECURITY_OPT = new Set(['no-new-privileges:true']);

export function validateComposeBody(bodyYaml: string): ComposeAllowlistResult {
  const errors: ComposeAllowlistError[] = [];
  const images: string[] = [];
  const serviceNames: string[] = [];
  const volumeNames: string[] = [];

  let doc: any;
  try {
    doc = yaml.load(bodyYaml);
  } catch (e) {
    return {
      valid: false,
      errors: [{ path: 'body', message: `YAML parse error: ${(e as Error).message}` }],
      images,
      serviceNames,
      volumeNames,
    };
  }

  if (!doc || typeof doc !== 'object') {
    return {
      valid: false,
      errors: [{ path: 'body', message: 'must be a YAML mapping' }],
      images,
      serviceNames,
      volumeNames,
    };
  }

  // Top-level
  for (const key of Object.keys(doc)) {
    if (key.startsWith('x-')) {
      // Only obacht-defined extensions allowed
      if (key !== 'x-obacht-data') {
        errors.push({ path: key, message: `forbidden extension '${key}' (only x-obacht-* extensions are allowed)` });
      }
      continue;
    }
    if (!ALLOWED_TOP_LEVEL.has(key)) {
      errors.push({ path: key, message: `forbidden top-level key '${key}'` });
    }
  }

  if (!doc.services || typeof doc.services !== 'object') {
    errors.push({ path: 'services', message: 'is required and must be a mapping' });
    return { valid: false, errors, images, serviceNames, volumeNames };
  }

  // Services
  const declaredVolumeNames = new Set<string>();
  if (doc.volumes && typeof doc.volumes === 'object') {
    for (const v of Object.keys(doc.volumes)) {
      declaredVolumeNames.add(v);
      volumeNames.push(v);
    }
  }

  for (const [svcName, svc] of Object.entries(doc.services as Record<string, any>)) {
    serviceNames.push(svcName);
    if (!svc || typeof svc !== 'object') {
      errors.push({ path: `services.${svcName}`, message: 'must be a mapping' });
      continue;
    }

    for (const [k, v] of Object.entries(svc)) {
      if (k.startsWith('x-')) continue;
      if (FORBIDDEN_SERVICE_KEYS_REASONS[k]) {
        errors.push({
          path: `services.${svcName}.${k}`,
          message: `forbidden key: ${FORBIDDEN_SERVICE_KEYS_REASONS[k]}`,
        });
        continue;
      }
      if (!ALLOWED_SERVICE_KEYS.has(k)) {
        errors.push({ path: `services.${svcName}.${k}`, message: `unknown key '${k}'` });
        continue;
      }

      // Per-key deeper checks
      if (k === 'image') {
        if (typeof v !== 'string' || v.length === 0) {
          errors.push({ path: `services.${svcName}.image`, message: 'must be a non-empty string' });
        } else {
          // strip any pinned @sha256:... — image-digest tracking is done from imageDigests map
          const ref = v.split('@')[0];
          if (!images.includes(ref)) images.push(ref);
        }
      } else if (k === 'restart') {
        if (typeof v !== 'string' || !ALLOWED_RESTART.has(v)) {
          errors.push({ path: `services.${svcName}.restart`, message: `must be one of ${[...ALLOWED_RESTART].join(', ')}` });
        }
      } else if (k === 'security_opt') {
        if (!Array.isArray(v)) {
          errors.push({ path: `services.${svcName}.security_opt`, message: 'must be an array' });
        } else {
          v.forEach((entry: unknown, i: number) => {
            if (typeof entry !== 'string' || !ALLOWED_SECURITY_OPT.has(entry)) {
              errors.push({
                path: `services.${svcName}.security_opt[${i}]`,
                message: `must be one of ${[...ALLOWED_SECURITY_OPT].join(', ')}`,
              });
            }
          });
        }
      } else if (k === 'volumes') {
        if (!Array.isArray(v)) {
          errors.push({ path: `services.${svcName}.volumes`, message: 'must be an array' });
        } else {
          v.forEach((mount: unknown, i: number) => {
            const mountPath = `services.${svcName}.volumes[${i}]`;
            if (typeof mount === 'string') {
              const [src] = mount.split(':');
              if (src.startsWith('/') || src.startsWith('.') || src.startsWith('~')) {
                errors.push({ path: mountPath, message: 'host bind mounts are forbidden — use a named volume declared in top-level volumes:' });
              } else if (!declaredVolumeNames.has(src)) {
                errors.push({ path: mountPath, message: `references undeclared named volume '${src}' — declare it in top-level volumes:` });
              }
            } else if (mount && typeof mount === 'object') {
              const m = mount as { type?: string; source?: string };
              if (m.type === 'bind') {
                errors.push({ path: mountPath, message: 'host bind mounts are forbidden — use a named volume' });
              } else if (m.source && !declaredVolumeNames.has(m.source)) {
                errors.push({ path: mountPath, message: `references undeclared named volume '${m.source}'` });
              }
            } else {
              errors.push({ path: mountPath, message: 'must be a string or object' });
            }
          });
        }
      } else if (k === 'cap_drop') {
        if (!Array.isArray(v)) errors.push({ path: `services.${svcName}.cap_drop`, message: 'must be an array' });
      }
    }
  }

  return { valid: errors.length === 0, errors, images, serviceNames, volumeNames };
}
