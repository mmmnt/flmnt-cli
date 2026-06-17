#!/bin/sh
set -e

REPO="mmmnt/flmnt-cli"
BIN="flmnt"

info() { printf '%s\n' "$*" >&2; }
err() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

if command -v curl >/dev/null 2>&1; then
	dl() { curl -fsSL "$1" -o "$2"; }
	fetch() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
	dl() { wget -qO "$2" "$1"; }
	fetch() { wget -qO - "$1"; }
else
	err "curl or wget is required"
fi

os=$(uname -s)
case "$os" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) err "unsupported OS: $os" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) err "unsupported architecture: $arch" ;;
esac

version="${FLMNT_VERSION:-}"
if [ -z "$version" ]; then
	version=$(fetch "https://api.github.com/repos/$REPO/releases/latest" |
		grep '"tag_name"' | head -1 |
		sed -E 's/.*"tag_name" *: *"([^"]+)".*/\1/')
	[ -n "$version" ] || err "could not determine the latest version"
fi
ver="${version#v}"

asset="${BIN}_${ver}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "Downloading $asset ($version)..."
dl "$base/$asset" "$tmp/$asset"
dl "$base/${BIN}_${ver}_checksums.txt" "$tmp/checksums.txt"

info "Verifying checksum..."
expected=$(grep " ${asset}\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || err "no checksum found for $asset"
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
	err "sha256sum or shasum is required"
fi
[ "$expected" = "$actual" ] || err "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp"

dir="${FLMNT_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
	if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		dir=/usr/local/bin
	else
		dir="$HOME/.local/bin"
	fi
fi
mkdir -p "$dir"
cp "$tmp/$BIN" "$dir/$BIN"
chmod 0755 "$dir/$BIN"

info "Installed $BIN to $dir/$BIN"
case ":$PATH:" in
*":$dir:"*) ;;
*) info "Add it to your PATH:  export PATH=\"$dir:\$PATH\"" ;;
esac
