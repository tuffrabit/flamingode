#!/usr/bin/env bash
set -euo pipefail

APP_NAME="flamingode"
OUTPUT_DIR="bin"
LDFLAGS="-s -w"
VERSION="${1:-}"

if [ -n "$VERSION" ]; then
    LDFLAGS="$LDFLAGS -X github.com/tuffrabit/flamingode/internal/version.Version=$VERSION"
fi

TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

for target in "${TARGETS[@]}"; do
    os="${target%/*}"
    arch="${target#*/}"
    if [ -n "$VERSION" ]; then
        output="${OUTPUT_DIR}/${APP_NAME}-${VERSION}-${os}-${arch}"
    else
        output="${OUTPUT_DIR}/${APP_NAME}-${os}-${arch}"
    fi

    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi

    echo "Building ${os}/${arch}..."
    GOOS="$os" GOARCH="$arch" go build -ldflags "$LDFLAGS" -o "$output" .
done

echo "Done. Binaries in ${OUTPUT_DIR}/"
