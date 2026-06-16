package benchutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
)

// Profile manages CPU and heap profiles for a single benchmark phase.
type Profile struct {
	dir     string
	phase   string
	cpuFile *os.File
}

// StartProfile begins CPU profiling for the named phase.
// Call Stop() when the phase completes to flush CPU profile and write a heap snapshot.
func StartProfile(dir, phase string) (*Profile, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create profile dir: %w", err)
	}

	cpuPath := filepath.Join(dir, phase+".cpu.prof")
	f, err := os.OpenFile(cpuPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // benchmark profile path is not user input
	if err != nil {
		return nil, fmt.Errorf("create cpu profile: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		cerr := f.Close()
		return nil, errors.Join(fmt.Errorf("start cpu profile: %w", err), cerr)
	}

	return &Profile{dir: dir, phase: phase, cpuFile: f}, nil
}

// Stop flushes the CPU profile and writes a heap snapshot.
// Returns the directory containing the profiles.
func (p *Profile) Stop() (string, error) {
	pprof.StopCPUProfile()
	if err := p.cpuFile.Close(); err != nil {
		return p.dir, fmt.Errorf("close cpu profile: %w", err)
	}

	runtime.GC()
	heapPath := filepath.Join(p.dir, p.phase+".heap.prof")
	f, err := os.OpenFile(heapPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // benchmark profile path is not user input
	if err != nil {
		return p.dir, fmt.Errorf("create heap profile: %w", err)
	}
	if err := pprof.WriteHeapProfile(f); err != nil {
		cerr := f.Close()
		return p.dir, errors.Join(fmt.Errorf("write heap profile: %w", err), cerr)
	}
	return p.dir, f.Close()
}
