#!/usr/bin/env tsx
/**
 * Validates every examples/*.yml against:
 *   1. JSON Schema (schema/manifest-v2.json) via ajv
 *   2. Lightweight TS validator (ts/manifest-v2.ts)
 *   3. Compose-body allowlist (ts/compose-allowlist.ts) for compose runtimes
 *
 * Exits non-zero on any failure.
 */
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import * as yaml from 'js-yaml';
import Ajv2019 from 'ajv/dist/2019.js';
import Ajv from 'ajv';
import addFormats from 'ajv-formats';
import { validateManifestV2 } from '../manifest-v2.js';
import { validateComposeBody } from '../compose-allowlist.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, '..', '..');
const schemaPath = join(repoRoot, 'schema', 'manifest-v2.json');
const examplesDir = join(repoRoot, 'examples');

const schema = JSON.parse(readFileSync(schemaPath, 'utf8'));
const ajv = new Ajv({ allErrors: true, strict: false });
addFormats(ajv as unknown as Ajv);
const validateSchema = ajv.compile(schema);

let failed = 0;
const files = readdirSync(examplesDir).filter((f) => f.endsWith('.yml') || f.endsWith('.yaml'));
if (files.length === 0) {
  console.error('no examples found');
  process.exit(1);
}

for (const file of files) {
  const path = join(examplesDir, file);
  const raw = readFileSync(path, 'utf8');
  const doc = yaml.load(raw) as Record<string, unknown>;

  const errors: string[] = [];

  // 1. JSON Schema
  if (!validateSchema(doc)) {
    for (const err of validateSchema.errors ?? []) {
      errors.push(`schema  ${err.instancePath || '/'}: ${err.message}`);
    }
  }

  // 2. TS validator
  const tsResult = validateManifestV2(doc);
  if (!tsResult.valid) {
    for (const e of tsResult.errors) errors.push(`ts      ${e.path}: ${e.message}`);
  }

  // 3. Compose body allowlist
  const runtime = (doc as { spec?: { runtime?: { type?: string; compose?: { body?: string } } } }).spec?.runtime;
  if (runtime?.type === 'compose' && typeof runtime.compose?.body === 'string') {
    const cbr = validateComposeBody(runtime.compose.body);
    if (!cbr.valid) {
      for (const e of cbr.errors) errors.push(`compose ${e.path}: ${e.message}`);
    }
  }

  if (errors.length === 0) {
    console.log(`  OK  ${file}`);
  } else {
    failed += 1;
    console.error(`  FAIL ${file}`);
    for (const e of errors) console.error(`        ${e}`);
  }
}

if (failed > 0) {
  console.error(`\n${failed} example(s) failed validation`);
  process.exit(1);
}
console.log(`\n${files.length} example(s) validated successfully`);
