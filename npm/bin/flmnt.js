#!/usr/bin/env node
import { spawnSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { binPath } from '../src/platform.js';

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const bin = binPath(root, process.platform, process.arch);

if (!existsSync(bin)) {
	console.error('flmnt binary not found — reinstall with `npm install -g @mmmnt/flmnt`');
	process.exit(1);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: 'inherit' });
if (result.error) {
	console.error(result.error.message);
	process.exit(1);
}
process.exit(result.status ?? 1);
