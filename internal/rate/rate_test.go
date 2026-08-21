package rate_test

import (
	"math"
	"testing"

	"github.com/SMoatassem/cgroups-stat/internal/cgroup"
	"github.com/SMoatassem/cgroups-stat/internal/rate"
)

// rec builds a minimal Record with just the CPU fields ComputeRates reads
// timestamp is in milliseconds (ComputeRates multiplies the delta by 1000)
func rec(path string, timestamp, usage, throttled, nrThrottled, nrPeriods int64) cgroup.Record {
	return cgroup.Record{
		AbsolutePath: path,
		Timestamp:    timestamp,
		CpuStat: map[string]int64{
			"usageUsec":     usage,
			"throttledUsec": throttled,
			"nrThrottled":   nrThrottled,
			"nrPeriods":     nrPeriods,
		},
	}
}

// findSample returns the sample for a path, and whether it was present.
func findSample(samples []rate.Sample, path string) (rate.Sample, bool) {
	for _, s := range samples {
		if s.Path == path {
			return s, true
		}
	}
	return rate.Sample{}, false
}

// Case 1: the normal case, 500000us of CPU over a 1000ms (1 000 000us) window
// is exactly half a core
func TestComputeRates_Normal(t *testing.T) {
	prev := []cgroup.Record{rec("/c", 0, 0, 0, 0, 0)}
	curr := []cgroup.Record{rec("/c", 1000, 500_000, 0, 0, 0)}

	s, ok := findSample(rate.ComputeRates(prev, curr), "/c")
	if !ok {
		t.Fatal("sample /c missing from output")
	}
	if !s.HasRate {
		t.Error("HasRate = false, should be true")
	}
	if s.CoresUsed != 0.5 {
		t.Errorf("CoresUsed = %v, should be 0.5", s.CoresUsed)
	}
}

// Case 2: counter reset. Usage going backwards means the cgroup was recreated,
// so the delta is meaningless and HasRate must be false
func TestComputeRates_CounterReset(t *testing.T) {
	prev := []cgroup.Record{rec("/c", 0, 1_000_000, 0, 0, 0)}
	curr := []cgroup.Record{rec("/c", 1000, 100, 0, 0, 0)}

	s, ok := findSample(rate.ComputeRates(prev, curr), "/c")
	if !ok {
		t.Fatal("sample /c missing from output")
	}
	if s.HasRate {
		t.Error("HasRate = true, should be false (counter went backwards)")
	}
}

// Case 3: zero elapsed time. Two snapshots with identical timestamps must not
// divide by zero into inf or nan
func TestComputeRates_ZeroElapsed(t *testing.T) {
	prev := []cgroup.Record{rec("/c", 1000, 0, 0, 0, 0)}
	curr := []cgroup.Record{rec("/c", 1000, 500_000, 0, 0, 0)}

	s, ok := findSample(rate.ComputeRates(prev, curr), "/c")
	if !ok {
		t.Fatal("sample /c missing from output")
	}
	if math.IsInf(s.CoresUsed, 0) || math.IsNaN(s.CoresUsed) {
		t.Errorf("CoresUsed = %v, should be a finite number", s.CoresUsed)
	}
}

// Case 4: new cgroup. Present in curr but not prev: no previous snapshot to
// diff against, so HasRate is false and nothing panics
func TestComputeRates_NewCgroup(t *testing.T) {
	prev := []cgroup.Record{}
	curr := []cgroup.Record{rec("/new", 1000, 500_000, 0, 0, 0)}

	s, ok := findSample(rate.ComputeRates(prev, curr), "/new")
	if !ok {
		t.Fatal("sample /new missing from output")
	}
	if s.HasRate {
		t.Error("HasRate = true, should be false (no previous snapshot)")
	}
}

// Case 5: disappeared cgroup. Present in prev, gone from curr: it must simply
// not appear in the output
func TestComputeRates_DisappearedCgroup(t *testing.T) {
	prev := []cgroup.Record{rec("/gone", 0, 0, 0, 0, 0)}
	curr := []cgroup.Record{}

	_ , ok := findSample(rate.ComputeRates(prev, curr), "/gone")

	if ok {
		t.Error("/gone present in output, should be dropped")
	}
}

// Case 6: no periods. When nr_periods is unchanged the throttle-per-period
// ratio would be 0/0; it must be reported as 0, not nan
func TestComputeRates_NoPeriods(t *testing.T) {
	prev := []cgroup.Record{rec("/c", 0, 0, 0, 2, 5)}
	curr := []cgroup.Record{rec("/c", 1000, 100, 0, 2, 5)}

	s, ok := findSample(rate.ComputeRates(prev, curr), "/c")
	if !ok {
		t.Fatal("sample /c missing from output")
	}
	if math.IsNaN(s.ThrottledPeriods) {
		t.Error("ThrottledPeriods = NaN, should be 0")
	}
	if s.ThrottledPeriods != 0 {
		t.Errorf("ThrottledPeriods = %v, should be 0", s.ThrottledPeriods)
	}
}
