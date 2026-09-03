#!/bin/bash
#
# Cross-compiles release binaries for the gh CLI extension.
#
# Invoked by cli/gh-extension-precompile as `./scripts/build-release.sh <tag>`.
# That action's build_script_override takes a PATH to an executable, not inline
# script content — passing the script body inline makes it try to exec a file
# named after the script's own first line and fail with exit 127.
set -euo pipefail

tag="${1:?usage: build-release.sh <tag>}"

mkdir -p dist

# Stamp the binary with the tag it is being cut from, so a release can never
# report a version that disagrees with its own tag.
ldflags="-X github.com/silouanwright/gh-comment/cmd.releaseVersion=${tag#v}"

# gh looks for dist/<name>-<goos>-<goarch>[.exe]; the names are a contract with
# the extension installer, not a convention.
targets=(
  "windows amd64 .exe"
  "windows 386   .exe"
  "windows arm64 .exe"
  "darwin  amd64"
  "darwin  arm64"
  "linux   amd64"
  "linux   386"
  "linux   arm"
  "linux   arm64"
  "freebsd amd64"
  "freebsd 386"
  "freebsd arm64"
)

for target in "${targets[@]}"; do
  read -r goos goarch ext <<<"$target"
  output="dist/gh-comment-${goos}-${goarch}${ext:-}"

  echo "building $output"
  GOOS="$goos" GOARCH="$goarch" go build -ldflags "$ldflags" -o "$output"
done

echo "built ${#targets[@]} binaries:"
ls -1 dist
