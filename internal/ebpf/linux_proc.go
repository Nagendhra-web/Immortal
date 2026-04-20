//go:build linux

package ebpf

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// newPlatformObserver on Linux returns a /proc-backed Observer. This is
// not a real eBPF probe yet; it is a dependency-free approximation that
// works on every Linux kernel without CAP_BPF or kernel headers.
//
// A real kprobe implementation (cilium/ebpf, attaching to
// tcp_retransmit_skb, do_fork, security_file_open) is tracked in issue
// #36 and will replace this file when it lands.
func newPlatformObserver() Observer {
	return &procObserver{}
}

type procObserver struct {
	lastSnap     *procSnap
	lastSnapAt   time.Time
}

type procSnap struct {
	outSegs      uint64
	retransSegs  uint64
	forks        uint64
	ctxtSwitches uint64
	procsRunning int
	openFiles    uint64
	maxFiles     uint64
}

func (p *procObserver) Read() (Snapshot, error) {
	curSnap, err := readProc()
	if err != nil {
		return Snapshot{}, err
	}
	curAt := time.Now()

	out := Snapshot{At: curAt, Source: "linux/proc"}
	// Always populate current gauges.
	if curSnap.maxFiles > 0 {
		out.OpenFilesPressure = float64(curSnap.openFiles) / float64(curSnap.maxFiles)
	}
	out.RunnableTasks = curSnap.procsRunning

	// Rates require a previous sample.
	if p.lastSnap != nil {
		interval := curAt.Sub(p.lastSnapAt).Seconds()
		if interval < 0.001 {
			interval = 0.001 // guard div-by-zero on very fast reads
		}
		segs := int64(curSnap.outSegs) - int64(p.lastSnap.outSegs)
		retrans := int64(curSnap.retransSegs) - int64(p.lastSnap.retransSegs)
		if segs > 0 && retrans >= 0 {
			out.TCPRetransmitsPer1k = uint64(retrans * 1000 / segs)
		}
		forks := int64(curSnap.forks) - int64(p.lastSnap.forks)
		if forks >= 0 {
			out.ForkRatePerSec = float64(forks) / interval
		}
		ctxt := int64(curSnap.ctxtSwitches) - int64(p.lastSnap.ctxtSwitches)
		if ctxt >= 0 {
			out.ContextSwitchesPerSec = float64(ctxt) / interval
		}
	}
	p.lastSnap = curSnap
	p.lastSnapAt = curAt
	return out, nil
}

func (p *procObserver) Signals(s Snapshot) SignalsReport {
	r := SignalsReport{Source: s.Source}
	// TCP retransmits: 0 per 1k is perfect; 100 per 1k is very bad.
	// Map linearly to 0..1 with saturation at 100.
	r.RetryStormRisk = clamp01(float64(s.TCPRetransmitsPer1k) / 100.0)
	// Open-file pressure is already 0..1. Saturate above 0.7 as risk=1.
	if s.OpenFilesPressure >= 0.7 {
		r.FDExhaustionRisk = 1
	} else {
		r.FDExhaustionRisk = s.OpenFilesPressure / 0.7
	}
	// Fork rate: 0 forks/sec is fine; sustained >500 is a red flag.
	r.RunawayWorkloadRisk = clamp01(s.ForkRatePerSec / 500.0)
	return r
}

func (p *procObserver) Source() string { return "linux/proc" }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// readProc aggregates the four /proc files we care about.
func readProc() (*procSnap, error) {
	s := &procSnap{}
	if err := readNetstat(s); err != nil {
		return nil, err
	}
	if err := readStat(s); err != nil {
		return nil, err
	}
	readFileNR(s) // best effort; not fatal
	return s, nil
}

// readNetstat parses /proc/net/netstat for TCPExt OutSegs + RetransSegs.
// Format: two header lines followed by two value lines per section.
func readNetstat(s *procSnap) error {
	f, err := os.Open("/proc/net/netstat")
	if err != nil {
		return fmt.Errorf("open /proc/net/netstat: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var header []string
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "TcpExt:") {
			continue
		}
		fields := strings.Fields(line)
		if header == nil {
			header = fields
			continue
		}
		for i, key := range header {
			if i >= len(fields) || i == 0 {
				continue
			}
			switch key {
			case "OutSegs":
				s.outSegs = atoiU(fields[i])
			case "RetransSegs":
				s.retransSegs = atoiU(fields[i])
			}
		}
		header = nil
	}
	return sc.Err()
}

// readStat parses /proc/stat for boot-time counters: "ctxt", "processes",
// "procs_running".
func readStat(s *procSnap) error {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return fmt.Errorf("open /proc/stat: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "ctxt":
			s.ctxtSwitches = atoiU(fields[1])
		case "processes":
			s.forks = atoiU(fields[1])
		case "procs_running":
			n, _ := strconv.Atoi(fields[1])
			s.procsRunning = n
		}
	}
	return sc.Err()
}

// readFileNR pulls total-open-files + max-open-files from /proc/sys/fs/file-nr.
// Best-effort: older kernels or non-Linux paths silently become zero.
func readFileNR(s *procSnap) {
	f, err := os.Open("/proc/sys/fs/file-nr")
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 3 {
			s.openFiles = atoiU(fields[0])
			s.maxFiles = atoiU(fields[2])
		}
	}
}

func atoiU(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}
