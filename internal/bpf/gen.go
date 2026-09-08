package bpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall" exec code/detect.c -- -I./bpf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
)

type Event struct {
	Pid uint32
	Comm [16]byte
}

func gen() {

	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal(err)
	}
	
	objs := execObjects{}
	if err := loadExecObjects(&objs, nil); err != nil {
		log.Fatalf("Objects loading error %v", err)
	}
	defer objs.Close()


	tp, err := link.Tracepoint("syscalls", "sys_exit_execve", objs.HandleExecve, nil)
	if err != nil {
		log.Fatalf("Tracepoint linking error : %v", err)
	}
	defer tp.Close()

	rd, err := ringbuf.NewReader(objs.Rb)
	if err != nil {
		log.Fatalf("Reader Initialization error: %v" , err)
	}
	defer rd.Close()

	fmt.Printf("Listening for execve..")

	for {
		record, err := rd.Read()
		if err != nil {
			log.Fatalf("Reading error : %v", err)
		} 

		var event Event
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
			log.Fatalf("decoding error: %v", err)
		}

		fmt.Printf("execve executed, PID : %d, commande %s\n", event.Pid, event.Comm)

	}
}
