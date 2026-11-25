// Copyright (c) lybox contributors
// SPDX-License-Identifier: BSD-3-Clause

package libtailscale

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tailscale.com/envknob"
	"tailscale.com/wgengine/magicsock"
)

// Power policy: trade path-maintenance chatter for battery life.
//
// Upstream keeps every peer under a 3s disco heartbeat for 45s after any
// traffic and re-runs STUN/netcheck every ~23s while a session is active. For a
// phone that is permanently in use by background apps (Syncthing, KDE Connect,
// chat), that means the radio never idles. See patches/tailscale.
//
// Individual features are toggled from the Settings screen, which writes
// <dataDir>/lybox_settings (one key=0|1 per line; missing keys are on):
//
//	cellular_derp     on cellular, use the DERP relay only (no UDP, no STUN)
//	relaxed_probing   when not DERP-only: 20s heartbeat, 5min direct retry/STUN
//	netcheck_cache    while DERP-only, re-measure DERP latency at most every 12h
//
// With every feature off the behaviour is stock Tailscale.
//
// <dataDir>/power_mode is an advanced override (adb) and wins when it is not "auto":
//
//	auto    (default) use the feature toggles above
//	derp    DERP only on every network
//	relaxed direct paths with relaxed probing on every network
//	stock   upstream behaviour
const (
	powerModeFile     = "power_mode"
	lyboxSettingsFile = "lybox_settings"
)

type lyboxFeatures struct {
	cellularDERP   bool
	relaxedProbing bool
	netcheckCache  bool
}

func readFeatures() lyboxFeatures {
	f := lyboxFeatures{cellularDERP: true, relaxedProbing: true, netcheckCache: true}
	if powerDataDir == "" {
		return f
	}
	b, err := os.ReadFile(filepath.Join(powerDataDir, lyboxSettingsFile))
	if err != nil {
		return f
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		on := strings.TrimSpace(v) != "0"
		switch strings.TrimSpace(k) {
		case "cellular_derp":
			f.cellularDERP = on
		case "relaxed_probing":
			f.relaxedProbing = on
		case "netcheck_cache":
			f.netcheckCache = on
		}
	}
	return f
}

// powerConfigKey identifies everything that influences the chosen policy.
func powerConfigKey() string {
	return fmt.Sprintf("%s|%+v", readPowerMode(), readFeatures())
}

var (
	powerDataDir string

	powerMu      sync.Mutex
	powerCurrent powerPolicy
	powerSet     bool
	powerIface   string // last non-empty default-route interface
	powerMode    string // mode that produced powerCurrent
)

type powerPolicy struct {
	derpOnly bool
	profile  magicsock.PowerProfile
}

var (
	policyStock = powerPolicy{}
	policyDERP  = powerPolicy{
		derpOnly: true,
		// No UDP socket exists, so there is nothing to keep alive or discover.
		profile: magicsock.PowerProfile{
			PeriodicSTUNInterval: -1,
			// Keep the home DERP chosen at first netcheck; re-measuring over
			// HTTPS costs ~1MB per run.
			NetcheckMinInterval: 12 * time.Hour,
		},
	}
	policyRelaxed = powerPolicy{
		profile: magicsock.PowerProfile{
			// Home/office NATs keep UDP mappings for >=30s; 20s keeps the
			// path warm with 3.5s of trust slack (see trustUDPAddrDurationNow).
			HeartbeatInterval:     20 * time.Second,
			UpgradeDirectInterval: 5 * time.Minute,
			PeriodicSTUNInterval:  5 * time.Minute,
		},
	}
)

// isCellularIface guesses from the Android interface name whether the default
// network is a mobile one. NetworkCapabilities would be exact, but the names
// used by Qualcomm, Exynos/Tensor and MediaTek modems are stable and this keeps
// the Kotlin/Go boundary unchanged.
func isCellularIface(ifname string) bool {
	ifname = strings.TrimPrefix(ifname, "v4-") // CLAT wraps the real interface
	for _, p := range []string{"rmnet", "ccmni", "pdp", "ppp", "wwan", "seth", "r_rmnet", "usb_rmnet", "dummy_rmnet"} {
		if strings.HasPrefix(ifname, p) {
			return true
		}
	}
	return false
}

func readPowerMode() string {
	if powerDataDir == "" {
		return "auto"
	}
	b, err := os.ReadFile(filepath.Join(powerDataDir, powerModeFile))
	if err != nil {
		return "auto"
	}
	return strings.ToLower(strings.TrimSpace(string(b)))
}

func choosePolicy(mode string, cellular bool, f lyboxFeatures) powerPolicy {
	switch mode {
	case "stock":
		return policyStock
	case "derp":
		return policyDERP
	case "relaxed":
		return policyRelaxed
	}
	// auto: follow the feature toggles.
	if cellular && f.cellularDERP {
		p := policyDERP
		if !f.netcheckCache {
			p.profile.NetcheckMinInterval = 0
		}
		return p
	}
	if f.relaxedProbing {
		return policyRelaxed
	}
	return policyStock
}

// applyPowerPolicy installs the policy that matches the new default network.
// It must run before the network change is delivered to netmon so that the
// rebind that follows already sees the new DERP-only setting.
func (b *backend) applyPowerPolicy(ifname string) {
	traceBackend.Store(b)
	if ifname == "" {
		return // offline: keep whatever was active
	}
	mode := powerConfigKey()
	cellular := isCellularIface(ifname)
	p := choosePolicy(readPowerMode(), cellular, readFeatures())

	powerMu.Lock()
	powerIface, powerMode = ifname, mode
	changedDERP := !powerSet || p.derpOnly != powerCurrent.derpOnly
	changed := !powerSet || p != powerCurrent
	powerCurrent, powerSet = p, true
	powerMu.Unlock()

	if !changed {
		return
	}
	log.Printf("power: mode=%s iface=%s cellular=%v derpOnly=%v profile=%+v", mode, ifname, cellular, p.derpOnly, p.profile)

	magicsock.SetPowerProfile(p.profile)
	if p.derpOnly {
		envknob.Setenv("TS_DEBUG_ALWAYS_USE_DERP", "1")
	} else {
		envknob.Setenv("TS_DEBUG_ALWAYS_USE_DERP", "")
	}
	// A DERP-only flip on an unchanged interface produces no netmon rebind,
	// so force one.
	if changedDERP && b.backend != nil {
		go func() {
			b.backend.DebugRebind()
			b.backend.DebugReSTUN()
		}()
	}
}

// watchPowerMode re-applies the policy when power_mode changes on disk, so
// experiments don't need a network flap or a restart.
func watchPowerMode() {
	for range time.Tick(5 * time.Second) {
		b := traceBackend.Load()
		if b == nil {
			continue
		}
		powerMu.Lock()
		iface, last := powerIface, powerMode
		powerMu.Unlock()
		if iface != "" && powerConfigKey() != last {
			b.applyPowerPolicy(iface)
		}
	}
}
