package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type record struct {
	absolutePath         string
	pids                 []int
	availableControllers []string
}

func parse_directory(path string, depth int, records []record) []record {
	files, err := os.ReadDir(path)

	if err != nil {
		fmt.Printf("error %v encountered\n", err)
		return records
	}

	var currentRecord record = record{}
	var controllers = []string{}
	var pids = []int{}

	content, err := os.ReadFile(path + "/cgroup.procs")

	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		pidsStr := strings.Fields(string(content))
		for _, pid := range pidsStr {
			pidInt, _ := strconv.Atoi(pid)
			pids = append(pids, pidInt)
		}
	}

	content, err = os.ReadFile(path + "/cgroup.controllers")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		controllers = strings.Fields(string(content))

	}

	for _, v := range files {
		for range depth {
			fmt.Printf("\t")
		}
		fmt.Printf("%v\n", v)
		if v.IsDir() {
			records = parse_directory(filepath.Join(path, v.Name()), depth+1, records)
		}
	}

	currentRecord.pids = pids
	currentRecord.absolutePath = path
	currentRecord.availableControllers = controllers

	records = append(records, currentRecord)

	return records
}

func main() {
	var dir string = "/sys/fs/cgroup"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	records := []record{}

	records = parse_directory(dir, 1, records)
	// for _, v := range records {
	// 	fmt.Println(v)
	// }
}
