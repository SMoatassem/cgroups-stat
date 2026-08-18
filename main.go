package main

import (
	"fmt"
	"os"
)


func parse_directory(path string, depth int) {
	files, err := os.ReadDir(path)
	
	if err != nil {
		fmt.Printf("error %v encountered\n", err)
		return
	}


	for _, v := range(files) {
		for range depth {
			fmt.Printf("\t")
		}
		fmt.Printf("%v\n", v)
		if v.IsDir() {
			parse_directory(path + "/" + v.Name(), depth + 1)
		}
	}
	
}


func main() {
	var dir string = "/sys/fs/cgroup"

	parse_directory(dir, 1)
}
