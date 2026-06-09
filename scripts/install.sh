#!/usr/bin/env bash
# Install the metalhost CLI on macOS or Linux.
# Usage:
#   curl -fsSL https://metalhost.net/install-cli.sh | bash
#   curl -fsSL https://metalhost.net/install-cli.sh | INSTALL_DIR=~/.local/bin bash
#   VERSION=v1.0.0-rc6 curl -fsSL https://metalhost.net/install-cli.sh | bash
set -euo pipefail

REPO="AES-Services/metalhost-cli"
BINARY="metalhost"

err() { echo "metalhost install: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

need_cmd curl
need_cmd tar
need_cmd uname

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) platform_os="darwin" ;;
  Linux)  platform_os="linux" ;;
  *) err "unsupported OS: $OS (macOS and Linux only)" ;;
esac

case "$ARCH" in
  x86_64|amd64)  platform_arch="x86_64" ;;
  aarch64|arm64) platform_arch="arm64" ;;
  armv7l|armv6l) platform_arch="armv7" ;;
  i386|i686)     platform_arch="i386" ;;
  *) err "unsupported CPU architecture: $ARCH" ;;
esac

if [ -n "${VERSION:-}" ]; then
  tag="${VERSION#v}"
  tag="v${tag}"
else
  tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=1" \
    | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$tag" ] || err "could not resolve latest release tag"
fi

version="${tag#v}"
archive="${BINARY}_${version}_${platform_os}_${platform_arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/${tag}"
archive_url="${base_url}/${archive}"
checksums_url="${base_url}/checksums.txt"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

echo "→ Installing metalhost ${tag} (${platform_os}/${platform_arch})"

curl -fsSL "$archive_url" -o "${tmpdir}/${archive}" \
  || err "download failed: ${archive_url}"

if curl -fsSL "$checksums_url" -o "${tmpdir}/checksums.txt" 2>/dev/null; then
  need_cmd sha256sum
  (
    cd "$tmpdir"
    grep -F " ${archive}" checksums.txt | sha256sum -c - >/dev/null \
      || err "checksum verification failed"
  )
  echo "→ Checksum verified"
fi

tar -xzf "${tmpdir}/${archive}" -C "$tmpdir" "$BINARY"
[ -f "${tmpdir}/${BINARY}" ] || err "archive did not contain ${BINARY} binary"

install_dir="${INSTALL_DIR:-}"
if [ -z "$install_dir" ]; then
  if [ -w "/usr/local/bin" ] 2>/dev/null; then
    install_dir="/usr/local/bin"
  else
    install_dir="${HOME}/.local/bin"
  fi
fi

mkdir -p "$install_dir"
target="${install_dir}/${BINARY}"

if [ -e "$target" ] && [ ! -w "$target" ]; then
  err "cannot write to ${target} — re-run with sudo or set INSTALL_DIR"
fi

install -m 0755 "${tmpdir}/${BINARY}" "$target"
echo "→ Installed ${target}"

if ! command -v "$BINARY" >/dev/null 2>&1; then
  case ":${PATH}:" in
    *":${install_dir}:"*) ;;
    *)
      echo ""
      echo "Add ${install_dir} to your PATH, then run: ${BINARY} version"
      echo "  export PATH=\"${install_dir}:\$PATH\""
      ;;
  esac
fi

"${target}" version
