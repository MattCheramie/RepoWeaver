#!/usr/bin/env bash
#
# Build cross-platform release artifacts for RepoWeaver.
#
# Produces, under dist/:
#   - one archive per target (tar.gz for unix, zip for windows) containing the
#     binary, README, LICENSE, app icon, and (on Linux) a .desktop launcher
#   - SHA256SUMS covering every archive
#
# The web build is pure-Go (CGO disabled), so all targets cross-compile from
# any host. The optional native "desktop" build is NOT produced here because it
# requires a platform webview toolchain; build it per-platform with
# `make desktop`.
#
# Usage: scripts/dist.sh [version]
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUT="dist"
LDFLAGS="-s -w -X main.version=${VERSION}"

# os/arch targets to build.
TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

rm -rf "$OUT"
mkdir -p "$OUT"

echo "Building RepoWeaver ${VERSION}"

for t in "${TARGETS[@]}"; do
  os="${t%/*}"
  arch="${t#*/}"
  name="repoweaver_${VERSION}_${os}_${arch}"
  stage="${OUT}/${name}"
  mkdir -p "$stage"

  bin="repoweaver"
  [ "$os" = "windows" ] && bin="repoweaver.exe"

  echo "  -> ${t}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "${stage}/${bin}" .

  # Common docs/assets.
  cp README.md LICENSE "$stage/"
  cp web/static/icon.svg "${stage}/repoweaver.svg"
  # Linux gets a .desktop launcher.
  if [ "$os" = "linux" ]; then
    cp packaging/repoweaver.desktop "$stage/"
  fi

  # Archive: zip for windows, tar.gz otherwise.
  if [ "$os" = "windows" ]; then
    ( cd "$OUT" && zip -qr "${name}.zip" "$name" )
  else
    tar -czf "${OUT}/${name}.tar.gz" -C "$OUT" "$name"
  fi
  rm -rf "$stage"
done

# Checksums over the produced archives.
( cd "$OUT" && sha256sum repoweaver_* > SHA256SUMS )

echo
echo "Artifacts in ${OUT}/:"
ls -1 "$OUT"
