import { spawnSync } from 'node:child_process';
import {
	chmodSync,
	mkdirSync,
	mkdtempSync,
	readFileSync,
	renameSync,
	rmSync,
	writeFileSync,
} from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assetName, binaryName, downloadURL, shouldInstall, target } from '../src/platform.js';

const REPO = 'mmmnt/flmnt-cli';

async function main() {
	const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
	const pkg = JSON.parse(readFileSync(path.join(root, 'package.json'), 'utf8'));
	if (!shouldInstall(pkg.version, process.env)) {
		console.log(`flmnt: skipping binary download (version ${pkg.version})`);
		return;
	}

	const { os: goos, arch } = target(process.platform, process.arch);
	const asset = assetName(pkg.version, goos, arch);
	const url = downloadURL(REPO, pkg.version, asset);
	console.log(`flmnt: downloading ${url}`);

	const res = await fetch(url, { redirect: 'follow' });
	if (!res.ok) {
		throw new Error(`download failed: HTTP ${res.status} for ${url}`);
	}
	const buf = Buffer.from(await res.arrayBuffer());

	const tmp = mkdtempSync(path.join(os.tmpdir(), 'flmnt-'));
	try {
		const archivePath = path.join(tmp, asset);
		writeFileSync(archivePath, buf);
		const extract = spawnSync('tar', ['-xf', archivePath, '-C', tmp], {
			stdio: 'inherit',
		});
		if (extract.status !== 0) {
			throw new Error('extraction failed (is `tar` on PATH?)');
		}

		const lib = path.join(root, 'lib');
		mkdirSync(lib, { recursive: true });
		const dest = path.join(lib, binaryName(goos));
		renameSync(path.join(tmp, binaryName(goos)), dest);
		if (goos !== 'windows') {
			chmodSync(dest, 0o755);
		}
		console.log(`flmnt: installed ${dest}`);
	} finally {
		rmSync(tmp, { recursive: true, force: true });
	}
}

main().catch((err) => {
	console.error(`flmnt: install failed — ${err.message}`);
	process.exit(1);
});
