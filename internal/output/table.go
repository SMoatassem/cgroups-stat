package output

import (
	"fmt"
	"path/filepath"
	"text/tabwriter"
	"os"
	"sort"
	"github.com/SMoatassem/cgroups-stat/internal/rate"
)

// humanBytes renders a byte count in binary units (KiB, MiB, ...)
func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// shortenPath trims the scan root so rows show the cgroup subtree, not the
// absolute /sys/fs/cgroup prefix repeated on every line
func shortenPath(path, root string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		if rel == "." {
			return "/"
		}
		return path
	}
	return rel
}

// printSamples renders the samples as an aligned table, sorted by sortKey
// (cores | throttle | periods | memory), most-notable first.
func PrintSamples(samples []rate.Sample, root, sortKey string, maxLines int) {
	sort.Slice(samples, func(i, j int) bool {
		switch sortKey {
		case "throttle":
			return samples[i].ThrottledFrac > samples[j].ThrottledFrac
		case "periods":
			return samples[i].ThrottledPeriods > samples[j].ThrottledPeriods
		case "memory":
			return samples[i].MemoryCurrent > samples[j].MemoryCurrent
		default: // cores
			return samples[i].CoresUsed > samples[j].CoresUsed
		}
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "CGROUP\tCORES\tQUOTA\tTHROTTLE%\tTHR_PERIODS\tMEMORY")
	for i := range min(len(samples), maxLines) {
		cores, throttle := "-", "-"
		s := samples[i]
		if s.HasRate {
			cores = fmt.Sprintf("%.3f", s.CoresUsed)
			throttle = fmt.Sprintf("%.1f%%", s.ThrottledFrac*100)
		}
		quota := "-"
		if s.QuotaFrac > 0 {
			quota = fmt.Sprintf("%.2f", s.QuotaFrac)
		}
		periods := "-"
		if s.ThrottledPeriods > 0 {
			periods = fmt.Sprintf("%.1f", s.ThrottledPeriods)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			shortenPath(s.Path, root), cores, quota, throttle, periods, humanBytes(s.MemoryCurrent))
	}
	w.Flush()
}