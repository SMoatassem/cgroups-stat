package exporter

import (
	"fmt"
	"net/http"

	"github.com/SMoatassem/cgroups-stat/internal/cgroup"
)

func ExportMetrics (w http.ResponseWriter, r *http.Request) {
	
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	records := []cgroup.Record{}
	fields := []string{"MemoryCurrent", "usageUsec", "throttledUsec", "nrThrottled", "nrPeriods"}

	// If prometheus is used, we only go to collect 
	// metrics from the /sys/fs/cgroup directory 
	records = cgroup.ParseDirectory("/sys/fs/cgroup", 1, records, false, false)

	for _, field := range(fields) {
		switch field {
			case "MemoryCurrent": 
				fmt.Fprintf(w, "# HELP cgstat_memory_usage_current_bytes Current memory usage in bytes\n")
				fmt.Fprintf(w, "# TYPE cgstat_memory_usage_current_bytes gauge\n")
			case "usageUsec":
				fmt.Fprintf(w, "# HELP cgstat_cpu_usage_seconds_total Current CPU usage in seconds\n")
				fmt.Fprintf(w, "# TYPE cgstat_cpu_usage_seconds_total counter\n")
			case "throttledUsec":
				fmt.Fprintf(w, "# HELP cgstat_cpu_throttled_seconds_total Current CPU throttle in seconds\n")
				fmt.Fprintf(w, "# TYPE cgstat_cpu_throttled_seconds_total counter\n")

			case "nrThrottled":
				fmt.Fprintf(w, "# HELP cgstat_cpu_throttled_periods_total Current CPU throttle periods\n")
				fmt.Fprintf(w, "# TYPE cgstat_cpu_throttled_periods_total counter\n")
			case "nrPeriods": 
				fmt.Fprintf(w, "# HELP cgstat_cpu_periods_total Current CPU periods\n")
				fmt.Fprintf(w, "# TYPE cgstat_cpu_periods_total counter\n")
		}
		for _, record := range(records) {
			switch field {
				case "MemoryCurrent":
					fmt.Fprintf(w, "cgstat_memory_usage_current_bytes{cgroup=%q} %v\n", record.AbsolutePath, record.MemoryCurrent)

				case "usageUsec":
					value, ok := record.CpuStat["usageUsec"]
					if ok {
						fmt.Fprintf(w, "cgstat_cpu_usage_seconds_total{cgroup=%q} %f\n", record.AbsolutePath, float64(value)/float64(1_000_000))
					}
					
				case "throttledUsec":
					value , ok := record.CpuStat["throttledUsec"]
					if ok {
						fmt.Fprintf(w, "cgstat_cpu_throttled_seconds_total{cgroup=%q} %f\n", record.AbsolutePath, float64(value)/float64(1_000_000))	
					}
				
				case "nrThrottled":
					value, ok := record.CpuStat["nrThrottled"]
					if ok {
						fmt.Fprintf(w, "cgstat_cpu_throttled_periods_total{cgroup=%q} %v\n", record.AbsolutePath, value)
					}
					
				case "nrPeriods":
					value, ok := record.CpuStat["nrPeriods"]
					if ok {
						fmt.Fprintf(w, "cgstat_cpu_periods_total{cgroup=%q} %v\n", record.AbsolutePath, value)
					}
			}
		}
		fmt.Fprintf(w, "\n")
	}
}