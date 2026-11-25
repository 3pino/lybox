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

# Run outside this module: on a fresh checkout the replace target doesn't exist
# yet and `go env` would fail while loading go.mod.
root=$(pwd)
gopath=$(cd "${TMPDIR:-/tmp}" && "$root/tool/go" env GOMODCACHE)
src="$gopath/tailscale.com@${mod_ver}"
if [[ ! -d $src ]]; then
  # Download from a scratch module so the (possibly dangling) replace
  # directive in our own go.mod can't get in the way.
  tmp=$(mktemp -d)
  (cd "$tmp" && "$root/tool/go" mod init scratch >/dev/null 2>&1 && "$root/tool/go" mod download "tailscale.com@${mod_ver}")
  rm -rf "$tmp"
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
