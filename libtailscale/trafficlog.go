// Copyright (c) lybox contributors
// SPDX-License-Identifier: BSD-3-Clause

package libtailscale

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tailscale.com/control/controlclient"
	"tailscale.com/derp"
	"tailscale.com/envknob"
	"tailscale.com/net/sockstats"
)

// Diagnostic: while <dataDir>/debug_traffic exists, log per-label socket byte
// deltas every 30s ("traffic: ..."). This answers "who is talking to the
// network while idle" without needing root/tcpdump. Labels are tailscale's own
// (ControlClientAuto, DERPHTTPClient, NetcheckClient, ...).
var trafficOnce sync.Once

func startTrafficLog() {
	trafficOnce.Do(func() { go trafficLogLoop(); go watchPowerMode() })
}

func trafficLogLoop() {
	prev := map[sockstats.Label]sockstats.SockStat{}
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		if powerDataDir == "" {
			continue
		}
		_, err := os.Stat(filepath.Join(powerDataDir, "debug_traffic"))
		on := err == nil
		if on {
			envknob.Setenv("TS_DEBUG_DISCO", "1")
		} else {
			envknob.Setenv("TS_DEBUG_DISCO", "")
		}
		updateCapture(on)
		derp.TraceFrames.Store(on)
		controlclient.TraceMap.Store(on)
		if !on {
			prev = map[sockstats.Label]sockstats.SockStat{}
			continue
		}
		cur := sockstats.Get()
		if cur == nil {
			continue
		}
		var parts []string
		for l, s := range cur.Stats {
			p := prev[l]
			dtx, drx := s.TxBytes-p.TxBytes, s.RxBytes-p.RxBytes
			if dtx|drx != 0 {
				parts = append(parts, fmt.Sprintf("%s tx=%d rx=%d", l, dtx, drx))
			}
			prev[l] = s
		}
		sort.Strings(parts)
		log.Printf("traffic[30s] cellular=%v: %s", cur.CurrentInterfaceCellular, strings.Join(parts, " | "))
	}
}
