package exporter

import (
	"fmt"
	"net/http"
	"math"

	"github.com/SMoatassem/cgroups-stat/internal/bpf"
	"github.com/SMoatassem/cgroups-stat/internal/cgroup"
)

func bucketBound(n int) float64 {
    return math.Ldexp(1, n) / 1e9  // 2^n nanoseconds, in seconds
}

func gather(rObjs bpf.RunqObj) (map[uint64]*[64]uint64, error) {

	res := make(map[uint64]*[64]uint64)
	objs := rObjs.Objs
	var key bpf.HistKey;
	var val uint64;


	iterator := objs.RunqHist.Iterate()
	for iterator.Next(&key, &val) {
		cgid := key.CgroupId
		bucket := key.Bucket
		
		if bucket >= 64 {
			continue
		}
		arr, ok := res[cgid]
		if !ok {
			arr = &[64]uint64{}
			res[cgid] = arr
		}
		arr[bucket] = val
	}
	
	if err := iterator.Err(); err != nil {
		return res, err
	}

	return res, nil
}

func ExportRunQLat (w http.ResponseWriter, r *http.Request, rObjs bpf.RunqObj) (error) {
	mapHis, _ := gather(rObjs)
	index, err := bpf.BuildCgroupIdx("/sys/fs/cgroup")

	if err != nil {
		return err
	}
    _, _ = fmt.Fprintln(w, "# HELP runqueue_latency_seconds Time tasks spent runnable before being scheduled.")
    _, _ = fmt.Fprintln(w, "# TYPE runqueue_latency_seconds histogram")

    for cgid, arr := range mapHis {
        name, ok := index[cgid]
        if !ok {
            name = fmt.Sprintf("unknown-%d", cgid)
        }

        var cumulative uint64
        var sum float64

        for n := 0; n < 64; n++ {
            bound := bucketBound(n)
            cumulative += arr[n]
			// we approximate the sum, given that we lose its
			// information when we put it in a bucket
            sum += float64(arr[n]) * bound

            _, _ = fmt.Fprintf(w, "runqueue_latency_seconds_bucket{cgroup=%q,le=\"%g\"} %d\n",
                name, bound, cumulative)
        }

        _, _ = fmt.Fprintf(w, "runqueue_latency_seconds_bucket{cgroup=%q,le=\"+Inf\"} %d\n", name, cumulative)
        _, _ = fmt.Fprintf(w, "runqueue_latency_seconds_sum{cgroup=%q} %g\n", name, sum)
        _, _ = fmt.Fprintf(w, "runqueue_latency_seconds_count{cgroup=%q} %d\n", name, cumulative)
    }

    return nil
}

func ExportMetrics (w http.ResponseWriter, r *http.Request) {

	records := []cgroup.Record{}
	fields := []string{"MemoryCurrent", "usageUsec", "throttledUsec", "nrThrottled", "nrPeriods"}

	// If prometheus is used, we only go to collect 
	// metrics from the /sys/fs/cgroup directory 
	records = cgroup.ParseDirectory("/sys/fs/cgroup", 1, records, false, false)

	for _, field := range(fields) {
		switch field {
			case "MemoryCurrent": 
				_, _ = fmt.Fprintf(w, "# HELP cgstat_memory_usage_current_bytes Current memory usage in bytes\n")
				_, _ = fmt.Fprintf(w, "# TYPE cgstat_memory_usage_current_bytes gauge\n")
			case "usageUsec":
				_, _ = fmt.Fprintf(w, "# HELP cgstat_cpu_usage_seconds_total Current CPU usage in seconds\n")
				_, _ = fmt.Fprintf(w, "# TYPE cgstat_cpu_usage_seconds_total counter\n")
			case "throttledUsec":
				_, _ = fmt.Fprintf(w, "# HELP cgstat_cpu_throttled_seconds_total Current CPU throttle in seconds\n")
				_, _ = fmt.Fprintf(w, "# TYPE cgstat_cpu_throttled_seconds_total counter\n")

			case "nrThrottled":
				_, _ = fmt.Fprintf(w, "# HELP cgstat_cpu_throttled_periods_total Current CPU throttle periods\n")
				_, _ = fmt.Fprintf(w, "# TYPE cgstat_cpu_throttled_periods_total counter\n")
			case "nrPeriods": 
				_, _ = fmt.Fprintf(w, "# HELP cgstat_cpu_periods_total Current CPU periods\n")
				_, _ = fmt.Fprintf(w, "# TYPE cgstat_cpu_periods_total counter\n")
		}
		for _, record := range(records) {
			switch field {
				case "MemoryCurrent":
					_, _ = fmt.Fprintf(w, "cgstat_memory_usage_current_bytes{cgroup=%q} %v\n", record.AbsolutePath, record.MemoryCurrent)

				case "usageUsec":
					value, ok := record.CpuStat["usageUsec"]
					if ok {
						_, _ = fmt.Fprintf(w, "cgstat_cpu_usage_seconds_total{cgroup=%q} %f\n", record.AbsolutePath, float64(value)/float64(1_000_000))
					}
					
				case "throttledUsec":
					value , ok := record.CpuStat["throttledUsec"]
					if ok {
						_, _ = fmt.Fprintf(w, "cgstat_cpu_throttled_seconds_total{cgroup=%q} %f\n", record.AbsolutePath, float64(value)/float64(1_000_000))	
					}
				
				case "nrThrottled":
					value, ok := record.CpuStat["nrThrottled"]
					if ok {
						_, _ = fmt.Fprintf(w, "cgstat_cpu_throttled_periods_total{cgroup=%q} %v\n", record.AbsolutePath, value)
					}
					
				case "nrPeriods":
					value, ok := record.CpuStat["nrPeriods"]
					if ok {
						_, _ = fmt.Fprintf(w, "cgstat_cpu_periods_total{cgroup=%q} %v\n", record.AbsolutePath, value)
					}
			}
		}
		_, _ = fmt.Fprintf(w, "\n")
	}
}


func ExporterWrapper(w http.ResponseWriter, r *http.Request, rObjs bpf.RunqObj) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	ExportMetrics(w, r)
	_ = ExportRunQLat(w, r, rObjs)
}
