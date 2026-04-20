//go:build !linux

package ebpf

// newPlatformObserver returns Nop on non-Linux systems. The rest of the
// engine keeps working identically; eBPF-derived signals simply report
// zero, so the advisor does not generate false positives on platforms
// where we cannot observe the kernel.
func newPlatformObserver() Observer { return Nop{} }
