
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

typedef struct {
    u64 cgroup_id;
    u32 bucket;
} hist_key;

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, hist_key);
    __type(value, u64); // This is simply a counter that gets incremented to build the histogram
} runq_hist SEC(".maps");


// we declare this as map aswell given that we cannot allocate in the
// restricted C code for eBPF (unecessary for the go code)
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, u32);   // pid
    __type(value, u64); // The timestamp when it becomes runnable and starts waiting for CPU time
} runb_tstmp SEC(".maps");

/*
BPF_PROGS allows us to directly get the elements of the array given by the kernel in the declared
arguments rather than keep splitting the array and casting the results to the corresponding format

NB: the format of tp_btf/sched_* can be found in /usr/src/linux-headers-$(uname -r)/include/trace/events/sched.h
*/


// Whenver we a process becomes runnable, whether for the first time (_new) or not, we add it's pid to runb_tstmp
SEC("tp_btf/sched_wakeup_new")
int BPF_PROG(add_to_map_new, struct task_struct *p) {
    // Helpers of map need to have pointers to the stack
    // otherwise, it's refused
    pid_t pid = BPF_CORE_READ(p, pid);
    // pid 0 is chosen whenever a core doesn't have any
    // work to do, meaningless here
    if (pid == 0) return 0;
    
    u64 tmstmp = bpf_ktime_get_ns();
    long res = bpf_map_update_elem(&runb_tstmp, &pid, &tmstmp, BPF_ANY);
    if (res < 0) {
        return -1;
    }
    
    return 0;

}

SEC("tp_btf/sched_wakeup")
int BPF_PROG(add_to_map, struct task_struct *p) {
    pid_t pid = BPF_CORE_READ(p, pid);
    if (pid == 0) return 0;
    
    u64 tmstmp = bpf_ktime_get_ns();
    long res = bpf_map_update_elem(&runb_tstmp, &pid, &tmstmp, BPF_ANY);
    if (res < 0) {
        return -1;
    }
    
    return 0;
}


SEC("tp_btf/sched_switch")
int BPF_PROG(fill_hist, bool preempt, struct task_struct *prev,
            struct task_struct *next, unsigned int prev_state) {
    
    u64 current_tmstmp = bpf_ktime_get_ns();
    if (prev_state == 0) {
        pid_t pid = BPF_CORE_READ(prev, pid);
        if (pid != 0) {
            long res = bpf_map_update_elem(&runb_tstmp, &pid, &current_tmstmp, BPF_ANY);
            if (res < 0) {
                return -1;
            }
        }
    }

    pid_t pid_next = BPF_CORE_READ(next, pid);
    if (pid_next == 0) return 0;

    u64 *tmstmp_runnable = bpf_map_lookup_elem(&runb_tstmp, &pid_next);
    if (!tmstmp_runnable) {
        return 0;
    }
    u64 start_ts = *tmstmp_runnable;

    long res = bpf_map_delete_elem(&runb_tstmp, &pid_next);
    if (res < 0) {
        return res;
    }

    u64 wait_time = current_tmstmp - start_ts;
    u32 bucket;
    if (wait_time == 0) {
        wait_time = 1;
    }
    u64 cgroup = BPF_CORE_READ(next, cgroups, dfl_cgrp, kn, id);

    // clzll is a built in function in the compiler
    // that counts the number of leading zeros, 63 minus x
    // thus happens to be exactly log_2(x) 
    bucket = 63 - __builtin_clzll(wait_time);
    
    // memset is essential here because sizeof(couple) = 16
    // and we would have 4 bytes of padding that can contain
    // old values potentially changing the hash calculations
    hist_key couple;
    __builtin_memset(&couple, 0, sizeof(couple));
    couple.cgroup_id = cgroup;
    couple.bucket = bucket;

    u64 *counter = bpf_map_lookup_elem(&runq_hist, &couple);
    if (counter) {
        __sync_fetch_and_add(counter, 1);
    } else {
        u64 one = 1;
        bpf_map_update_elem(&runq_hist, &couple, &one, BPF_ANY);
    }


    return 0;
}

char LICENSE[] SEC("license") = "GPL";