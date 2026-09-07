package main

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -type hist_key runqlat code/bucket.c -- -I./bpf

import (
	"fmt"
	"log"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)


type RunQhist struct {
	Key runqlatHistKey
	Value uint64
}

type RunbTstmp struct {
	Key uint32
	Value uint64
}

func main() {

	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal(err)
	}
	
	var objs runqlatObjects
	if err := loadRunqlatObjects(&objs, nil); err != nil {
		log.Fatalf("Objects loading error %v", err)
	}
	defer objs.Close()

	programs := []*ebpf.Program{
		objs.AddToMap,
		objs.AddToMapNew,
		objs.FillHist,
	}

	for _, program := range(programs) {
		l, err := link.AttachTracing(link.TracingOptions{Program: program})
		if err != nil {
			log.Fatal(err)
		}
		defer l.Close()
	}

	for {
		var key runqlatHistKey;
		var val uint64;

		iterator := objs.RunqHist.Iterate()
		for iterator.Next(&key, &val) {
			fmt.Printf("cgroupId: %v, bucket %v: value: %v\n", key.CgroupId, key.Bucket, val)
		}
		if err := iterator.Err(); err != nil {
			log.Fatal(err)
		}

		time.Sleep(15 * time.Second)
	}

}
