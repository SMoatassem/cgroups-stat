package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type record struct {
	absolutePath         string
	pids                 []int
	availableControllers []string
	hasChildren          bool
	
	memoryCurrent		 int64
	hasMemoryCurrent	 bool
	memoryMax			 int64
	hasMemoryMax		 bool

	cpuQuota			 int64
	cpuPeriod		 	 int64
	hasCPUQuota			 bool

	timestamp			 int64
	cpuStat 			 map[string]int64
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

	content, err := os.ReadFile(filepath.Join(path , "/cgroup.procs"))

	if err != nil {
		if vOpt {fmt.Printf("error: %v\n", err)}
	} else {
		pidsStr := strings.Fields(string(content))
		for _, pid := range pidsStr {
			pidInt, _ := strconv.Atoi(pid)
			pids = append(pids, pidInt)
		}
	}

	content, err = os.ReadFile(filepath.Join(path ,"/cgroup.controllers"))
	if err != nil {
		if vOpt {fmt.Printf("error: %v\n", err)}
	} else {
		controllers = strings.Fields(string(content))

	}

	// Parse metrics
	content, err = os.ReadFile(filepath.Join(path , "memory.current"))
	if err != nil {
		if vOpt {fmt.Printf("error: %v\n", err)}
		currentRecord.hasMemoryCurrent = false
	} else {
		memoryCurr, _ := strconv.ParseInt(strings.TrimSpace(string(content)),10, 64)
		currentRecord.hasMemoryCurrent = true
		currentRecord.memoryCurrent = memoryCurr
	}

	content, err = os.ReadFile(filepath.Join(path , "memory.max"))
	if err != nil {
		if vOpt {fmt.Printf("error: %v\n", err)}
		currentRecord.hasMemoryMax = false
	} else if strings.TrimSpace(string(content)) == "max" {
		currentRecord.hasMemoryMax = false
	} else {
		memoryMax, _ := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
		currentRecord.hasMemoryMax = true
		currentRecord.memoryMax = memoryMax
	}

	content, err = os.ReadFile(filepath.Join(path ,"cpu.max"))
	if err != nil {
		if vOpt {fmt.Printf("error: %v\n", err)}
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

	content, err = os.ReadFile(filepath.Join(path ,"cpu.stat"))
	if err != nil {
		if vOpt {fmt.Printf("error: %v\n", err)}
	} else {
		currentRecord.timestamp = time.Now().UnixMilli()
		// When we will add the Ticker, or a sleep call to collect metrics
		// Again and regather data, there will necessary be some sort  of
		// latency between two distinct path, the timestamp makes the counting
		// more credible

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		for _, line := range(lines) {
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

func main() {
	tree := flag.Bool("tree", false, "View the cgroup hierarchy as a tree")
	path := flag.String("path", "/sys/fs/cgroup", "Modify the starting point of parsing")
	verbose := flag.Bool("v", false, "Enable verbosity to view errors")
	flag.Parse()

	var dir string = *path

	records := []record{}

	records = parseDirectory(dir, 1, records, *tree, *verbose)
	for _, v := range records {
		fmt.Println(v)
	}
}
