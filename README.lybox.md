# LyBox

Battery-frugal fork of [tailscale-android](https://github.com/tailscale/tailscale-android)
for an always-on, background-heavy phone.

Package id `dev.lybox.ts` (installs next to the official app).

## What differs from upstream

* `patches/tailscale/*.patch` is applied to the pinned `tailscale.com` module by
  `scripts/tailscale-src.sh` (run by `make`), output in `third_party/tailscale`
  (git-ignored; `go.mod` has a `replace` for it). It adds `magicsock.PowerProfile`:
  heartbeat / direct-upgrade / periodic-STUN intervals and a netcheck cache.
* `libtailscale/power.go` picks a profile per network: cellular -> DERP only,
  Wi-Fi -> direct paths with relaxed probing. Override with
  `adb shell run-as dev.lybox.ts sh -c 'echo MODE > files/power_mode'` where MODE is
  `auto` (default) | `derp` | `relaxed` | `stock`; applied on the next network change.

## Build

    make apk        # -> tailscale-debug.apk (debug-signed)

Needs JDK 17+, Android SDK/NDK; Go is fetched by `tool/go`.

## Measuring

See `scripts/` for nothing yet; the ad-hoc procedure used so far: `dumpsys battery unplug`,
`dumpsys batterystats --reset`, screen off, wait, read per-UID `pwi`/`nt` and
"Mobile radio active" from `dumpsys batterystats`.
