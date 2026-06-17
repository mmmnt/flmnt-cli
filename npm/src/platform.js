import path from 'node:path';

const NODE_OS = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
const NODE_ARCH = { x64: 'amd64', arm64: 'arm64' };

export function target(platform, arch) {
	const os = NODE_OS[platform];
	const goarch = NODE_ARCH[arch];
	if (!os || !goarch) {
		throw new Error(`unsupported platform: ${platform}/${arch}`);
	}
	return { os, arch: goarch };
}

export function archiveExt(os) {
	return os === 'windows' ? 'zip' : 'tar.gz';
}

export function assetName(version, os, arch) {
	return `flmnt_${version}_${os}_${arch}.${archiveExt(os)}`;
}

export function binaryName(os) {
	return os === 'windows' ? 'flmnt.exe' : 'flmnt';
}

export function binPath(root, platform, arch) {
	const { os } = target(platform, arch);
	return path.join(root, 'lib', binaryName(os));
}

export function downloadURL(repo, version, asset) {
	return `https://github.com/${repo}/releases/download/v${version}/${asset}`;
}

export function shouldInstall(version, env) {
	if (env.FLMNT_SKIP_DOWNLOAD) {
		return false;
	}
	return !version.includes('-dev');
}
