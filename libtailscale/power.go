// Copyright (c) lybox contributors
// SPDX-License-Identifier: BSD-3-Clause

package libtailscale

import (
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
// The mode is read from <dataDir>/power_mode on every network change:
//
//	auto    (default) cellular: DERP only; Wi-Fi: direct with relaxed probing
//	derp    DERP only on every network
//	relaxed direct paths with relaxed probing on every network
//	stock   upstream behaviour
const powerModeFile = "power_mode"

var (
	powerDataDir string

	powerMu      sync.Mutex
	powerCurrent powerPolicy
	powerSet     bool
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

func choosePolicy(mode string, cellular bool) powerPolicy {
	switch mode {
	case "stock":
		return policyStock
	case "derp":
		return policyDERP
	case "relaxed":
		return policyRelaxed
	default: // auto
		if cellular {
			return policyDERP
		}
		return policyRelaxed
	}
}

// applyPowerPolicy installs the policy that matches the new default network.
// It must run before the network change is delivered to netmon so that the
// rebind that follows already sees the new DERP-only setting.
func (b *backend) applyPowerPolicy(ifname string) {
	if ifname == "" {
		return // offline: keep whatever was active
	}
	mode := readPowerMode()
	cellular := isCellularIface(ifname)
	p := choosePolicy(mode, cellular)

	powerMu.Lock()
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
