#!/usr/bin/env bash
# Materialise a patched copy of the tailscale.com module at third_party/tailscale.
# go.mod points `replace tailscale.com` there. Idempotent: re-runs only when the
# pinned module version or the patch set changed.
set -euo pipefail
cd "$(dirname "$0")/.."

# Resolve the pinned module without the replace directive in the way.
mod_ver=$(awk '$1=="tailscale.com" && $2 ~ /^v/ {print $2; exit}' go.mod)
stamp="third_party/tailscale/.lybox-stamp"
want="${mod_ver} $(cat patches/tailscale/*.patch | sha256sum | cut -d' ' -f1)"
if [[ -f $stamp && "$(cat "$stamp")" == "$want" ]]; then exit 0; fi

# On a fresh checkout the replace target doesn't exist yet, so `go` can't load
# our go.mod. Point it at a scratch module file for these two commands.
root=$(pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
printf 'module scratch\n\ngo 1.21\n' > "$tmp/go.mod"
gopath=$(GOFLAGS="-modfile=$tmp/go.mod" ./tool/go env GOMODCACHE)
src="$gopath/tailscale.com@${mod_ver}"
if [[ ! -d $src ]]; then
  GOFLAGS="-modfile=$tmp/go.mod" ./tool/go mod download "tailscale.com@${mod_ver}"
fi
[[ -d $src ]] || { echo "module cache missing $src" >&2; exit 1; }

rm -rf third_party/tailscale
mkdir -p third_party
cp -r "$src" third_party/tailscale
chmod -R u+w third_party/tailscale
for p in patches/tailscale/*.patch; do
  patch -p1 -d third_party/tailscale < "$p"
done
echo "$want" > "$stamp"
