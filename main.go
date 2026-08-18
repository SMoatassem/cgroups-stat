package main

import (
	"fmt"
	"os"
	"strings"
	"strconv"
)

type record struct {
	absolute_path string;
	pids []int;
	available_controllers []string;
}

func parse_directory(path string, depth int, records []record) []record {
	files, err := os.ReadDir(path)

	if err != nil {
		fmt.Printf("error %v encountered\n", err)
		return records
	}

	var current_record record = record{}
	var controllers = []string{}
	var pids = []int{}


	content, err := os.ReadFile(path + "/cgroup.procs")

	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		pids_str := strings.Fields(string(content))
		fmt.Printf("============= LE CONTENU DE CGROUP.PROCS POUR %v EST:\n %v\n\n", path, string(content))
		for _ ,pid := range(pids_str) {
			pid_int, _ := strconv.Atoi(pid)
			pids = append(pids, pid_int)
		}
	}

	content, err = os.ReadFile(path + "/cgroup.controllers")
	if err != nil {
		fmt.Printf("error: %v\n", err)
	} else {
		controllers = strings.Fields(string(content))

	}

	for _, v := range(files) {
		for range depth {
			fmt.Printf("\t")
		}
		fmt.Printf("%v\n", v)
		if v.IsDir() {
			records = parse_directory(path + v.Name(), depth + 1, records)
		}
	}
	
	current_record.pids = pids
	current_record.absolute_path = path
	current_record.available_controllers = controllers

	records = append(records, current_record)
	
	return records
}


func main() {
	var dir string = "/sys/fs/cgroup"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	
	records := []record{}

	records = parse_directory(dir, 1, records)
	for _, v := range(records) {
		fmt.Println(v)
	}
}
