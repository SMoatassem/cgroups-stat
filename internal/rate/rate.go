package rate

import (
	"github.com/SMoatassem/cgroups-stat/internal/cgroup"
)

type Sample struct {
	Path             string
	CoresUsed        float64
	ThrottledFrac    float64
	ThrottledPeriods float64
	MemoryCurrent    int64
	QuotaFrac        float64 // only when hasCPUQuota
	HasRate          bool    // false when no previous snapshot existed
}


func ComputeRates(prev []cgroup.Record, curr []cgroup.Record) []Sample {
	// Convert slices to maps to facilitate lookup rather than a opt for a nested loop
	var prevMap map[string]cgroup.Record = make(map[string]cgroup.Record)
	var currMap map[string]cgroup.Record = make(map[string]cgroup.Record)

	for _,v := range prev {
		prevMap[v.AbsolutePath] = v
	}

	for _,v := range curr {
		currMap[v.AbsolutePath] = v
	}

	samples := []Sample{}
	for key, value := range currMap {
		newSample := Sample{}

		currentRecord := value
		prevRecord, ok := prevMap[key]

		newSample.Path = key
		newSample.HasRate = ok
		newSample.MemoryCurrent = currentRecord.MemoryCurrent
		if currentRecord.HasCPUQuota {
			newSample.QuotaFrac = float64(currentRecord.CpuQuota) / float64(currentRecord.CpuPeriod)
		}

		if ok {
			deltaUsec := (currentRecord.Timestamp - prevRecord.Timestamp) * 1000
			deltaUsage := currentRecord.CpuStat["usageUsec"] - prevRecord.CpuStat["usageUsec"]
			deltaThrottled := currentRecord.CpuStat["throttledUsec"] - prevRecord.CpuStat["throttledUsec"]
			deltaThrottledPeriods := currentRecord.CpuStat["nrThrottled"] - prevRecord.CpuStat["nrThrottled"]
			deltaPeriods := currentRecord.CpuStat["nrPeriods"] - prevRecord.CpuStat["nrPeriods"]
			
			if deltaUsec > 0 && min(deltaUsage, deltaThrottled, deltaThrottledPeriods, deltaPeriods) >= 0 {
				newSample.CoresUsed = float64(deltaUsage) / float64(deltaUsec)
				newSample.ThrottledFrac = float64(deltaThrottled) / float64(deltaUsec)
				if deltaPeriods > 0 {
					newSample.ThrottledPeriods = float64(deltaThrottledPeriods) / float64(deltaPeriods)
				}
			} else {
				// Containing having been reset per example, there is no prev
				newSample.HasRate = false
			}




		}

		samples = append(samples, newSample)

	}
	return samples
}