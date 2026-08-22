package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SMoatassem/cgroups-stat/internal/cgroup"
	"github.com/SMoatassem/cgroups-stat/internal/exporter"
	"github.com/SMoatassem/cgroups-stat/internal/output"
	"github.com/SMoatassem/cgroups-stat/internal/rate"
)


func main() {
	tree := flag.Bool("tree", false, "View the cgroup hierarchy as a tree")
	path := flag.String("path", "/sys/fs/cgroup", "Modify the starting point of parsing")
	verbose := flag.Bool("v", false, "Enable verbosity to view errors")
	sortKey := flag.String("sort", "cores", "Sort table by: cores | throttle | periods | memory")
	maxLines := flag.Int("n", 20 , "Change the number of lines shown in the table")
	prom := flag.Bool("prom", false, "Enable http server for Prometheus exportation")
	flag.Parse()

	const clearScreen = "\033[H\033[2J\033[3J"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var dir string = *path

	if ! *prom {
		records := []cgroup.Record{}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		
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
	} else {
		mux := http.NewServeMux()
		mux.HandleFunc("/metrics", exporter.ExportMetrics)
		srv := &http.Server{
			Addr:              ":9100",
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func () {
			fmt.Println("starting HTTP server.. ")
			err := srv.ListenAndServe()

			if err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP server couldn't start, error %v\n", err)
				stop()
			}

		}()

		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}

	

	
}
