#!/usr/bin/env tsx
/**
 * Negative tests for the compose-allowlist validator.
 * Asserts that forbidden top-level + per-service keys are rejected.
 */
import { validateComposeBody } from '../compose-allowlist.js';

interface Case {
  name: string;
  body: string;
  expectInvalid: boolean;
  expectError?: RegExp;
}

const cases: Case[] = [
  {
    name: 'minimal valid',
    body: `
services:
  web:
    image: ghost:5-alpine
    restart: unless-stopped
`,
    expectInvalid: false,
  },
  {
    name: 'forbidden privileged',
    body: `
services:
  web:
    image: ghost:5-alpine
    privileged: true
`,
    expectInvalid: true,
    expectError: /defeats sandboxing/,
  },
  {
    name: 'forbidden host bind mount',
    body: `
services:
  web:
    image: ghost:5-alpine
    volumes:
      - /etc/passwd:/etc/passwd
`,
    expectInvalid: true,
    expectError: /host bind/,
  },
  {
    name: 'forbidden ports key',
    body: `
services:
  web:
    image: ghost:5-alpine
    ports:
      - "8080:80"
`,
    expectInvalid: true,
    expectError: /host port mapping/,
  },
  {
    name: 'forbidden cap_add',
    body: `
services:
  web:
    image: ghost:5-alpine
    cap_add: [NET_ADMIN]
`,
    expectInvalid: true,
    expectError: /closed list/,
  },
  {
    name: 'forbidden network_mode',
    body: `
services:
  web:
    image: ghost:5-alpine
    network_mode: host
`,
    expectInvalid: true,
    expectError: /bundle isolation/,
  },
  {
    name: 'forbidden top-level extension',
    body: `
x-malicious: { foo: bar }
services:
  web:
    image: ghost:5-alpine
`,
    expectInvalid: true,
    expectError: /forbidden extension/,
  },
  {
    name: 'allowed x-obacht-data extension',
    body: `
services:
  web:
    image: ghost:5-alpine
    volumes:
      - content:/var/lib/ghost/content
volumes:
  content:
    x-obacht-data: true
`,
    expectInvalid: false,
  },
  {
    name: 'restart=always forbidden',
    body: `
services:
  web:
    image: ghost:5-alpine
    restart: always
`,
    expectInvalid: true,
    expectError: /must be one of/,
  },
  {
    name: 'undeclared named volume',
    body: `
services:
  web:
    image: ghost:5-alpine
    volumes:
      - mystery:/data
`,
    expectInvalid: true,
    expectError: /undeclared named volume/,
  },
  {
    name: 'security_opt allowlist',
    body: `
services:
  web:
    image: ghost:5-alpine
    security_opt:
      - "no-new-privileges:true"
`,
    expectInvalid: false,
  },
  {
    name: 'security_opt non-allowlisted',
    body: `
services:
  web:
    image: ghost:5-alpine
    security_opt:
      - "label:disable"
`,
    expectInvalid: true,
    expectError: /no-new-privileges/,
  },
];

let failed = 0;
for (const c of cases) {
  const result = validateComposeBody(c.body);
  const ok = result.valid !== c.expectInvalid;
  let detailFailure = '';
  if (ok && c.expectInvalid && c.expectError) {
    const matched = result.errors.some((e) => c.expectError!.test(e.message));
    if (!matched) {
      detailFailure = `expected error matching ${c.expectError} but got: ${result.errors.map((e) => e.message).join('; ')}`;
    }
  }
  if (!ok || detailFailure) {
    failed += 1;
    console.error(`  FAIL ${c.name}`);
    if (!ok) {
      console.error(`        expected ${c.expectInvalid ? 'invalid' : 'valid'} but was ${result.valid ? 'valid' : 'invalid'}`);
      result.errors.forEach((e) => console.error(`        - ${e.path}: ${e.message}`));
    } else if (detailFailure) {
      console.error(`        ${detailFailure}`);
    }
  } else {
    console.log(`  OK   ${c.name}`);
  }
}

if (failed > 0) {
  console.error(`\n${failed} case(s) failed`);
  process.exit(1);
}
console.log(`\n${cases.length} case(s) passed`);
