package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
)

type sample struct {
	path             string
	coresUsed        float64
	throttledFrac    float64
	throttledPeriods float64
	memoryCurrent    int64
	quotaFrac        float64 // only when hasCPUQuota
	hasRate          bool    // false when no previous snapshot existed
}

type record struct {
	absolutePath         string
	pids                 []int
	availableControllers []string
	hasChildren          bool

	memoryCurrent    int64
	hasMemoryCurrent bool
	memoryMax        int64
	hasMemoryMax     bool

	cpuQuota    int64
	cpuPeriod   int64
	hasCPUQuota bool

	timestamp int64
	cpuStat   map[string]int64
}

func parseDirectory(path string, depth int, records []record, treeOpt bool, vOpt bool) []record {
	files, err := os.ReadDir(path)

	if err != nil {
		fmt.Printf("error %v encountered\n", err)
		return records
	}

	var currentRecord record = record{}
	var controllers = []string{}
	var pids = []int{}
	var hasChildren bool = false
	var cpuStatRecord = make(map[string]int64)

	content, err := os.ReadFile(filepath.Join(path, "/cgroup.procs"))

	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
	} else {
		pidsStr := strings.Fields(string(content))
		for _, pid := range pidsStr {
			pidInt, _ := strconv.Atoi(pid)
			pids = append(pids, pidInt)
		}
	}

	content, err = os.ReadFile(filepath.Join(path, "/cgroup.controllers"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
	} else {
		controllers = strings.Fields(string(content))

	}

	// Parse metrics
	content, err = os.ReadFile(filepath.Join(path, "memory.current"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
		currentRecord.hasMemoryCurrent = false
	} else {
		memoryCurr, _ := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		currentRecord.hasMemoryCurrent = true
		currentRecord.memoryCurrent = memoryCurr
	}

	content, err = os.ReadFile(filepath.Join(path, "memory.max"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
		currentRecord.hasMemoryMax = false
	} else if strings.TrimSpace(string(content)) == "max" {
		currentRecord.hasMemoryMax = false
	} else {
		memoryMax, _ := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		currentRecord.hasMemoryMax = true
		currentRecord.memoryMax = memoryMax
	}

	content, err = os.ReadFile(filepath.Join(path, "cpu.max"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
	} else {
		cpuMaxInfo := strings.Fields(string(content))
		if len(cpuMaxInfo) >= 2 {
			cpuWindow, _ := strconv.ParseInt(cpuMaxInfo[1], 10, 64)
			currentRecord.cpuPeriod = cpuWindow
			if cpuMaxInfo[0] == "max" {
				currentRecord.hasCPUQuota = false
			} else {
				cpuMaxWindow, _ := strconv.ParseInt(cpuMaxInfo[0], 10, 64)
				currentRecord.hasCPUQuota = true
				currentRecord.cpuQuota = cpuMaxWindow
			}
		}
	}

	content, err = os.ReadFile(filepath.Join(path, "cpu.stat"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
	} else {
		currentRecord.timestamp = time.Now().UnixMilli()
		// When we will add the Ticker, or a sleep call to collect metrics
		// Again and regather data, there will necessary be some sort  of
		// latency between two distinct path, the timestamp makes the counting
		// more credible

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		for _, line := range lines {
			splittedLine := strings.Fields(line)
			// fmt.Printf("%v\n\n\n", splittedLine)
			if len(splittedLine) > 1 {
				switch splittedLine[0] {
				case "usage_usec":
					value, _ := strconv.ParseInt(splittedLine[1], 10, 64)
					cpuStatRecord["usageUsec"] = value

				case "nr_throttled":
					value, _ := strconv.ParseInt(splittedLine[1], 10, 64)
					cpuStatRecord["nrThrottled"] = value

				case "nr_periods":
					value, _ := strconv.ParseInt(splittedLine[1], 10, 64)
					cpuStatRecord["nrPeriods"] = value

				case "throttled_usec":
					value, _ := strconv.ParseInt(splittedLine[1], 10, 64)
					cpuStatRecord["throttledUsec"] = value

				default:
					continue
				}
			}

		}
	}

	for _, v := range files {
		if treeOpt {
			for range depth {
				fmt.Printf("\t")
			}
			fmt.Printf("%v\n", v)
		}

		if v.IsDir() {
			hasChildren = true
			records = parseDirectory(filepath.Join(path, v.Name()), depth+1, records, treeOpt, vOpt)
		}
	}

	currentRecord.pids = pids
	currentRecord.absolutePath = path
	currentRecord.availableControllers = controllers
	currentRecord.hasChildren = hasChildren
	currentRecord.cpuStat = cpuStatRecord

	records = append(records, currentRecord)

	return records
}

func computeRates(prev []record, curr []record) []sample {
	// Convert slices to maps to facilitate lookup rather than a opt for a nested loop
	var prevMap map[string]record = make(map[string]record)
	var currMap map[string]record = make(map[string]record)

	for _,v := range prev {
		prevMap[v.absolutePath] = v
	}

	for _,v := range curr {
		currMap[v.absolutePath] = v
	}

	samples := []sample{}
	for key, value := range currMap {
		newSample := sample{}

		currentRecord := value
		prevRecord, ok := prevMap[key]

		newSample.path = key
		newSample.hasRate = ok
		newSample.memoryCurrent = currentRecord.memoryCurrent
		if currentRecord.hasCPUQuota {
			newSample.quotaFrac = float64(currentRecord.cpuQuota) / float64(currentRecord.cpuPeriod)
		}

		if ok {
			deltaUsec := (currentRecord.timestamp - prevRecord.timestamp) * 1000
			deltaUsage := currentRecord.cpuStat["usageUsec"] - prevRecord.cpuStat["usageUsec"]
			deltaThrottled := currentRecord.cpuStat["throttledUsec"] - prevRecord.cpuStat["throttledUsec"]
			deltaThrottledPeriods := currentRecord.cpuStat["nrThrottled"] - prevRecord.cpuStat["nrThrottled"]
			deltaPeriods := currentRecord.cpuStat["nrPeriods"] - prevRecord.cpuStat["nrPeriods"]

			newSample.coresUsed = float64(deltaUsage) / float64(deltaUsec)
			newSample.throttledFrac = float64(deltaThrottled) / float64(deltaUsec)
			if deltaPeriods > 0 {
				newSample.throttledPeriods = float64(deltaThrottledPeriods) / float64(deltaPeriods)
			}

		}

		samples = append(samples, newSample)

	}
	return samples
}

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
func printSamples(samples []sample, root, sortKey string) {
	sort.Slice(samples, func(i, j int) bool {
		switch sortKey {
		case "throttle":
			return samples[i].throttledFrac > samples[j].throttledFrac
		case "periods":
			return samples[i].throttledPeriods > samples[j].throttledPeriods
		case "memory":
			return samples[i].memoryCurrent > samples[j].memoryCurrent
		default: // cores
			return samples[i].coresUsed > samples[j].coresUsed
		}
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "CGROUP\tCORES\tQUOTA\tTHROTTLE%\tTHR_PERIODS\tMEMORY")
	for _, s := range samples {
		cores, throttle := "-", "-"
		if s.hasRate {
			cores = fmt.Sprintf("%.3f", s.coresUsed)
			throttle = fmt.Sprintf("%.1f%%", s.throttledFrac*100)
		}
		quota := "-"
		if s.quotaFrac > 0 {
			quota = fmt.Sprintf("%.2f", s.quotaFrac)
		}
		periods := "-"
		if s.throttledPeriods > 0 {
			periods = fmt.Sprintf("%.1f", s.throttledPeriods)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			shortenPath(s.path, root), cores, quota, throttle, periods, humanBytes(s.memoryCurrent))
	}
	w.Flush()
}

func main() {
	tree := flag.Bool("tree", false, "View the cgroup hierarchy as a tree")
	path := flag.String("path", "/sys/fs/cgroup", "Modify the starting point of parsing")
	verbose := flag.Bool("v", false, "Enable verbosity to view errors")
	sortKey := flag.String("sort", "cores", "Sort table by: cores | throttle | periods | memory")
	flag.Parse()

	const clearScreen = "\033[H\033[2J\033[3J"
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var dir string = *path

	records := []record{}

	prev := parseDirectory(dir, 1, records, *tree, *verbose)
	if *verbose {
		for _, v := range prev {
			fmt.Println(v)
		}
	}
	
	for {
		select {
		case <- ticker.C: 
			curr := parseDirectory(dir, 1, records, *tree, *verbose)
			
			samples := computeRates(prev, curr)
			
			fmt.Print(clearScreen)
			printSamples(samples, dir, *sortKey)
		
			prev = curr
		
		case <- ctx.Done():
			return
		}
	}
	

	
}
