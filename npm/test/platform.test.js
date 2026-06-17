import path from 'node:path';
import { describe, it, expect } from 'vitest';
import {
	target,
	archiveExt,
	assetName,
	binaryName,
	binPath,
	downloadURL,
	shouldInstall,
} from '../src/platform.js';

describe('target', () => {
	it('maps darwin/arm64 to goreleaser darwin/arm64', () => {
		expect(target('darwin', 'arm64')).toEqual({ os: 'darwin', arch: 'arm64' });
	});
	it('maps linux/x64 to goreleaser linux/amd64', () => {
		expect(target('linux', 'x64')).toEqual({ os: 'linux', arch: 'amd64' });
	});
	it('maps win32/x64 to goreleaser windows/amd64', () => {
		expect(target('win32', 'x64')).toEqual({ os: 'windows', arch: 'amd64' });
	});
	it('throws on an unsupported platform/arch', () => {
		expect(() => target('aix', 'ppc64')).toThrow('unsupported platform: aix/ppc64');
	});
});

describe('archiveExt', () => {
	it('uses zip for windows', () => {
		expect(archiveExt('windows')).toBe('zip');
	});
	it('uses tar.gz for non-windows', () => {
		expect(archiveExt('linux')).toBe('tar.gz');
	});
});

describe('assetName', () => {
	it('builds the goreleaser tar.gz asset name for unix', () => {
		expect(assetName('1.2.3', 'darwin', 'arm64')).toBe('flmnt_1.2.3_darwin_arm64.tar.gz');
	});
	it('builds the goreleaser zip asset name for windows', () => {
		expect(assetName('1.2.3', 'windows', 'amd64')).toBe('flmnt_1.2.3_windows_amd64.zip');
	});
});

describe('binaryName', () => {
	it('appends .exe on windows', () => {
		expect(binaryName('windows')).toBe('flmnt.exe');
	});
	it('has no extension on unix', () => {
		expect(binaryName('linux')).toBe('flmnt');
	});
});

describe('binPath', () => {
	it('resolves the unix binary under lib/', () => {
		expect(binPath('/pkg', 'linux', 'x64')).toBe(path.join('/pkg', 'lib', 'flmnt'));
	});
	it('resolves the windows binary under lib/', () => {
		expect(binPath('/pkg', 'win32', 'x64')).toBe(path.join('/pkg', 'lib', 'flmnt.exe'));
	});
});

describe('shouldInstall', () => {
	it('skips the dev placeholder version', () => {
		expect(shouldInstall('0.0.0-dev', {})).toBe(false);
	});
	it('installs a real release version', () => {
		expect(shouldInstall('1.2.3', {})).toBe(true);
	});
	it('skips when FLMNT_SKIP_DOWNLOAD is set', () => {
		expect(shouldInstall('1.2.3', { FLMNT_SKIP_DOWNLOAD: '1' })).toBe(false);
	});
});

describe('downloadURL', () => {
	it('builds the github release download url with a v-prefixed tag', () => {
		expect(downloadURL('mmmnt/flmnt-cli', '1.2.3', 'flmnt_1.2.3_linux_amd64.tar.gz')).toBe(
			'https://github.com/mmmnt/flmnt-cli/releases/download/v1.2.3/flmnt_1.2.3_linux_amd64.tar.gz'
		);
	});
});
