// Copyright (c) lybox contributors
// SPDX-License-Identifier: BSD-3-Clause

package libtailscale

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tailscale.com/net/packet"
)

// Diagnostic packet summary, enabled together with debug_traffic. Aggregates
// every packet crossing the tun / WireGuard / disco capture points into
// per-flow counters and logs the top flows every 30s ("flows[30s]").

var traceBackend atomic.Pointer[backend]

type flowKey struct {
	path  packet.CapturePath
	src   string
	dst   string
	proto string
	flags string
}

type flowStat struct{ pkts, bytes int }

var (
	flowMu        sync.Mutex
	flows         = map[flowKey]*flowStat{}
	captureActive bool
)

func captureCallback(path packet.CapturePath, _ time.Time, data []byte, _ packet.CaptureMeta) {
	k := flowKey{path: path}
	if path == packet.PathDisco {
		k.proto = "disco"
	} else {
		var p packet.Parsed
		p.Decode(data)
		k.src, k.dst = p.Src.String(), p.Dst.String()
		k.proto = p.IPProto.String()
		if p.IPProto == 6 { // TCP
			k.flags = tcpFlagString(p.TCPFlags)
		}
		// Collapse ephemeral ports: keep only the lower one (the service).
		sp, dp := p.Src.Port(), p.Dst.Port()
		if sp != 0 && dp != 0 && (sp == 8080 || dp == 8080) {
			// keep both ports for the disnans server so connections can be told apart
			k.src, k.dst = fmt.Sprintf("%s:%d", p.Src.Addr(), sp), fmt.Sprintf("%s:%d", p.Dst.Addr(), dp)
		} else if sp != 0 && dp != 0 {
			if sp < dp {
				k.src, k.dst = fmt.Sprintf("%s:%d", p.Src.Addr(), sp), p.Dst.Addr().String()
			} else {
				k.src, k.dst = p.Src.Addr().String(), fmt.Sprintf("%s:%d", p.Dst.Addr(), dp)
			}
		}
	}
	flowMu.Lock()
	s := flows[k]
	if s == nil {
		s = &flowStat{}
		flows[k] = s
	}
	s.pkts++
	s.bytes += len(data)
	flowMu.Unlock()
}

func pathName(p packet.CapturePath) string {
	switch p {
	case packet.FromLocal:
		return "app->tun"
	case packet.FromPeer:
		return "peer->us"
	case packet.SynthesizedToLocal:
		return "ts->app"
	case packet.SynthesizedToPeer:
		return "ts->peer"
	case packet.PathDisco:
		return "disco"
	}
	return fmt.Sprint(uint8(p))
}

// updateCapture installs/removes the hook as the debug_traffic marker comes and
// goes, and flushes the flow table to the log.
func updateCapture(on bool) {
	b := traceBackend.Load()
	if b != nil && b.engine != nil {
		if on && !captureActive {
			b.engine.InstallCaptureHook(captureCallback)
			captureActive = true
		} else if !on && captureActive {
			b.engine.InstallCaptureHook(nil)
			captureActive = false
		}
	}
	flowMu.Lock()
	cur := flows
	flows = map[flowKey]*flowStat{}
	flowMu.Unlock()
	if !on || len(cur) == 0 {
		return
	}
	type kv struct {
		k flowKey
		s *flowStat
	}
	var all []kv
	for k, s := range cur {
		all = append(all, kv{k, s})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].s.pkts > all[j].s.pkts })
	var sb strings.Builder
	for i, e := range all {
		if i >= 14 {
			break
		}
		fmt.Fprintf(&sb, "\n  %-8s %-22s -> %-22s %-5s %-4s pkts=%d bytes=%d",
			pathName(e.k.path), e.k.src, e.k.dst, e.k.proto, e.k.flags, e.s.pkts, e.s.bytes)
	}
	log.Printf("flows[window]:%s", sb.String())
}

func tcpFlagString(f packet.TCPFlag) string {
	var sb strings.Builder
	for _, x := range []struct {
		f packet.TCPFlag
		c string
	}{{packet.TCPSyn, "S"}, {packet.TCPFin, "F"}, {packet.TCPRst, "R"}, {packet.TCPPsh, "P"}, {packet.TCPAck, "."}} {
		if f&x.f != 0 {
			sb.WriteString(x.c)
		}
	}
	return sb.String()
}
