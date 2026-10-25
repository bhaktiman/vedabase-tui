#!/bin/sh
# Installs the latest vedabase release for Linux or macOS.
#
#   curl -fsSL https://github.com/bhaktiman/vedabase-tui/releases/latest/download/install.sh | sh
#
# Set INSTALL_DIR to choose where the binary goes (default ~/.local/bin) and
# VERSION to pin a release tag such as v0.1.0 (default: the latest).
set -eu

repo="bhaktiman/vedabase-tui"
install_dir="${INSTALL_DIR:-$HOME/.local/bin}"
version="${VERSION:-latest}"

fail() { echo "vedabase install: $*" >&2; exit 1; }

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "unsupported system $(uname -s); download a binary from https://github.com/$repo/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported CPU $(uname -m)" ;;
esac

if [ "$version" = latest ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/$version"
fi
asset="vedabase_${os}_${arch}.tar.gz"

if command -v sha256sum >/dev/null 2>&1; then
  sha() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
  sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
  fail "need sha256sum or shasum to verify the download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading $asset ($version)…"
curl -fsSL "$base/$asset" -o "$tmp/$asset" || fail "download failed: $base/$asset"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "download failed: $base/checksums.txt"

want="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
[ -n "$want" ] || fail "$asset is not listed in checksums.txt"
[ "$(sha "$tmp/$asset")" = "$want" ] || fail "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp" vedabase
mkdir -p "$install_dir"
mv "$tmp/vedabase" "$install_dir/vedabase"
chmod 755 "$install_dir/vedabase"

echo "Installed $("$install_dir/vedabase" --version) to $install_dir/vedabase"
case ":$PATH:" in
  *":$install_dir:"*) echo "Run: vedabase" ;;
  *) echo "$install_dir is not on your PATH. Add this to your shell profile, then run vedabase:"
     echo "  export PATH=\"$install_dir:\$PATH\"" ;;
esac
