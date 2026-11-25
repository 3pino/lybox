# LyBox

<img src="app-icon.svg" width="96" align="right" alt="LyBox icon">

A battery-friendly Tailscale client for Android. It is a fork of
[tailscale-android](https://github.com/tailscale/tailscale-android) that keeps the tailnet
connected all day while waking the mobile radio far less often.

LyBox is an unofficial project and is not affiliated with or endorsed by Tailscale Inc.
It connects to the normal Tailscale control plane, so a Tailscale account is required.

## Download

Grab the APK from the [Releases page](https://github.com/3pino/lybox/releases/latest)
(`lybox-vX.Y.Z-arm64.apk`, arm64 phones only, Android 8.0+).

- The package id is `dev.lybox.ts`, so it installs **next to** the official app.
  Only one VPN can be active at a time, so turn the official app off first.
- Releases are signed with a fixed key. Updates install over the previous version.
- Sign in with the browser flow or an auth key, as in the official app.

## What is different from the official app

The official app keeps its network paths warm: a heartbeat every 3 seconds, periodic STUN
probes, and frequent re-checks of direct paths. Each one wakes the modem, so with the
screen off the mobile radio stays active a large part of the time.

LyBox changes how often these probes run. Everything else (WireGuard, MagicDNS, ACLs,
Tailscale SSH as a client, the UI) is upstream code.

| | Official | LyBox |
|---|---|---|
| Cellular | direct paths + 3 s heartbeat + STUN | **DERP relay only**, no STUN, netcheck cached for 12 h |
| Wi-Fi | direct paths + 3 s heartbeat | direct paths, heartbeat 20 s, direct-path retry 5 min, STUN 5 min |
| Package id | `com.tailscale.ipn` | `dev.lybox.ts` |
| ABIs | many | arm64-v8a only |

Trade-offs:

- Over cellular, traffic goes through a Tailscale DERP relay, so latency is higher
  (typically tens of milliseconds) and throughput is limited by the relay.
- A dead path is noticed more slowly than with a 3 s heartbeat.
- Incoming connections keep working: the tunnel stays up and the device stays reachable.

## Settings: turn LyBox features on or off

Open **Settings** and scroll to **LyBox power saving**. Each LyBox change has its own switch,
so you can use LyBox-only behaviour, the official behaviour, or any mix. Changes apply within
a few seconds, without reconnecting.

| Switch (default on) | What it does when on | When off |
|---|---|---|
| Relay only on mobile data | On cellular, use DERP relays only, no UDP or STUN | Direct paths and STUN, as official |
| Relaxed path probing | Heartbeat 20 s, direct-path retry and STUN every 5 min | 3 s heartbeat and frequent STUN, as official |
| Cache relay latency checks | While relay-only, re-measure DERP latency at most every 12 h | Re-measure as official |

With all switches off, the network behaviour is the same as the official app.

### Advanced: `power_mode` override

For experiments, a file in the app's private directory overrides the switches. It is re-read
every few seconds.

```sh
adb shell run-as dev.lybox.ts sh -c 'echo MODE > files/power_mode'
```

| MODE | Behaviour |
|---|---|
| `auto` (default) | follow the switches above |
| `derp` | always DERP only |
| `relaxed` | always direct paths with relaxed probing |
| `stock` | upstream behaviour (for comparison) |

## How it is built

- `patches/tailscale/*.patch` is applied to the pinned `tailscale.com` module by
  `scripts/tailscale-src.sh` (run by `make`). The result goes to `third_party/tailscale`
  (git-ignored) and `go.mod` has a `replace` pointing at it. The patch adds
  `magicsock.PowerProfile` and makes the heartbeat, direct-path, STUN and netcheck
  intervals configurable.
- `libtailscale/power.go` chooses the profile for the current network (interface name based).
- `libtailscale/trafficlog.go` and `capture.go` are diagnostics. They are off unless a
  `files/debug_traffic` marker file exists.
- `scripts/measure/exp.sh` is a harness for comparing packet counts and battery stats between modes.

### Build locally

```sh
make apk            # debug build -> tailscale-debug.apk
make lybox-release  # release build -> lybox-release.apk
```

You need JDK 17+ (CI uses 21) and the Android SDK with NDK. Go is fetched by `tool/go`.
The release build is signed only if `LYBOX_KEYSTORE`, `LYBOX_KEYSTORE_PASSWORD`,
`LYBOX_KEY_ALIAS` and `LYBOX_KEY_PASSWORD` are set; `LYBOX_VERSION` sets the version name.

### Release

Pushing a tag such as `v0.1.1` runs `.github/workflows/release.yml`, which builds a signed
arm64 APK and attaches it to a GitHub Release. The signing key is stored in repository secrets.

## Not implemented yet

An app allow-list UI (the upstream split-tunnelling screen exists), and removal of unused upstream features such as
exit nodes and Taildrop.

## License and credits

BSD 3-Clause, same as upstream. See [LICENSE](LICENSE) and [PATENTS](PATENTS).
The original README is kept as [README-tailscale.md](README-tailscale.md).
"Tailscale" is a trademark of Tailscale Inc.; LyBox uses its own name and icon.
