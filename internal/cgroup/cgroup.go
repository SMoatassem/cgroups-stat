package cgroup

import (
	"os"
	"fmt"
	"strconv"
	"strings"
	"time"
	"path/filepath"

)

type Record struct {
	AbsolutePath         string
	Pids                 []int
	AvailableControllers []string
	HasChildren          bool

	MemoryCurrent    int64
	HasMemoryCurrent bool
	MemoryMax        int64
	HasMemoryMax     bool

	CpuQuota    int64
	CpuPeriod   int64
	HasCPUQuota bool

	Timestamp int64
	CpuStat   map[string]int64
}




func ParseDirectory(path string, depth int, records []Record, treeOpt bool, vOpt bool) []Record {
	files, err := os.ReadDir(path)

	if err != nil {
		fmt.Printf("error %v encountered\n", err)
		return records
	}

	var currentRecord Record = Record{}
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
		currentRecord.HasMemoryCurrent = false
	} else {
		memoryCurr, _ := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		currentRecord.HasMemoryCurrent = true
		currentRecord.MemoryCurrent = memoryCurr
	}

	content, err = os.ReadFile(filepath.Join(path, "memory.max"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
		currentRecord.HasMemoryMax = false
	} else if strings.TrimSpace(string(content)) == "max" {
		currentRecord.HasMemoryMax = false
	} else {
		memoryMax, _ := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		currentRecord.HasMemoryMax = true
		currentRecord.MemoryMax = memoryMax
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
			currentRecord.CpuPeriod = cpuWindow
			if cpuMaxInfo[0] == "max" {
				currentRecord.HasCPUQuota = false
			} else {
				cpuMaxWindow, _ := strconv.ParseInt(cpuMaxInfo[0], 10, 64)
				currentRecord.HasCPUQuota = true
				currentRecord.CpuQuota = cpuMaxWindow
			}
		}
	}

	content, err = os.ReadFile(filepath.Join(path, "cpu.stat"))
	if err != nil {
		if vOpt {
			fmt.Printf("error: %v\n", err)
		}
	} else {
		currentRecord.Timestamp = time.Now().UnixMilli()
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
			records = ParseDirectory(filepath.Join(path, v.Name()), depth+1, records, treeOpt, vOpt)
		}
	}

	relativePath := strings.TrimPrefix(path, "/sys/fs/cgroup")
	if relativePath == "" {
		relativePath = "/"
	}
	currentRecord.Pids = pids
	currentRecord.AbsolutePath = relativePath
	currentRecord.AvailableControllers = controllers
	currentRecord.HasChildren = hasChildren
	currentRecord.CpuStat = cpuStatRecord

	records = append(records, currentRecord)

	return records
}
