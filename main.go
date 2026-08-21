package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"cgroups-stat/internal/cgroup"
	"cgroups-stat/internal/rate"
	"cgroups-stat/internal/output"
)


func main() {
	tree := flag.Bool("tree", false, "View the cgroup hierarchy as a tree")
	path := flag.String("path", "/sys/fs/cgroup", "Modify the starting point of parsing")
	verbose := flag.Bool("v", false, "Enable verbosity to view errors")
	sortKey := flag.String("sort", "cores", "Sort table by: cores | throttle | periods | memory")
	maxLines := flag.Int("n", 20 , "Change the number of lines shown in the table")
	flag.Parse()

	const clearScreen = "\033[H\033[2J\033[3J"
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var dir string = *path

	records := []cgroup.Record{}

	prev := cgroup.ParseDirectory(dir, 1, records, *tree, *verbose)
	if *verbose {
		for _, v := range prev {
			fmt.Println(v)
		}
	}
	
	for {
		select {
		case <- ticker.C: 
			curr := cgroup.ParseDirectory(dir, 1, records, *tree, *verbose)
			
			samples := rate.ComputeRates(prev, curr)
			
			fmt.Print(clearScreen)
			output.PrintSamples(samples, dir, *sortKey, *maxLines)
		
			prev = curr
		
		case <- ctx.Done():
			return
		}
	}
	

	
}
