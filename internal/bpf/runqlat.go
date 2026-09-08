package bpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" -type hist_key runqlat code/bucket.c -- -I./bpf

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

type HistKey = runqlatHistKey

type RunQhist struct {
	Key runqlatHistKey
	Value uint64
}

type RunbTstmp struct {
	Key uint32
	Value uint64
}

type RunqObj struct {
	Objs runqlatObjects
	Links []link.Link
	// We must save references to links aswell
	// Otherwise we might lose them due to the 
	// Go garbage collector
}

func BuildCgroupIdx(root string) (map[uint64]string ,error) {
	res := make(map[uint64]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		
		if err != nil || !d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		st := info.Sys().(*syscall.Stat_t)
		name := strings.TrimPrefix(path, root)
		if name == "" {
			name = "/"
		}
		res[st.Ino] = name
		return nil
	})

	return res, err
}


func InitEbpfRunQ() (RunqObj, error){

	if err := rlimit.RemoveMemlock(); err != nil {
		return RunqObj{}, fmt.Errorf("removing memlock:%v", err)
	}
	
	var objs runqlatObjects
	if err := loadRunqlatObjects(&objs, nil); err != nil {
		return RunqObj{}, fmt.Errorf("objects loading:%v", err)
	}
	// defer objs.Close()

	programs := []*ebpf.Program{
		objs.AddToMap,
		objs.AddToMapNew,
		objs.FillHist,
	}

	links := []link.Link{}
	for _, program := range(programs) {
		l, err := link.AttachTracing(link.TracingOptions{Program: program})
		if err != nil {
			return RunqObj{}, fmt.Errorf("link tracing:%v", err)
		}
		// defer l.Close()
		links = append(links, l)
	}


	res := RunqObj{Objs: objs, Links: links}

	return res, nil
}
