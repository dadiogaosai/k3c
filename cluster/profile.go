package cluster

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/philipparndt/go-logger"

	"k3c/config"
	"k3c/runtime"
)

// PodSample is one pod's resource accounting at a single instant, read
// straight from the node's cgroup v2 hierarchy.
type PodSample struct {
	// CPUUsec is the cumulative on-CPU time of the whole pod (all its
	// containers) since the pod sandbox was created, in microseconds. It is
	// the kernel's own accounting (cpu.stat usage_usec) — the same figure the
	// scheduler bills — so it is exact, not sampled like cAdvisor/metrics.
	CPUUsec int64 `json:"cpu_usec"`
	// MemWorkingSet is the pod working-set in bytes: memory.current minus the
	// reclaimable inactive file cache (matching kubelet's workingSet metric).
	MemWorkingSet int64 `json:"mem_ws"`
	// MemCurrent is the raw memory.current in bytes.
	MemCurrent int64 `json:"mem_current"`
}

// Snapshot is one sampling tick: every pod's accounting, stamped with the
// host wall-clock time (in Unix milliseconds) at which k3c read the tick off
// the node stream. Stamping on the host keeps all snapshots on the same clock
// as any consumer correlating them with Kubernetes events.
type Snapshot struct {
	TimeMillis int64                `json:"t_ms"`
	Pods       map[string]PodSample `json:"pods"`
}

// profileScript samples the cgroup hierarchy on the node. It walks the
// per-pod cgroups under kubepods (cgroupfs driver layout) and prints one line
// per pod — "uid cpu_usec mem_current inactive_file" — followed by a "==="
// delimiter, every INTERVAL seconds. Reading happens entirely on the node in
// one long-lived shell, so there is no per-tick exec overhead.
//
// The pod-level cpu.stat aggregates all of the pod's container cgroups, so a
// single read per pod is both correct and cheap.
const profileScript = `INT=%s
while true; do
  for d in /sys/fs/cgroup/kubepods/*/pod*/ /sys/fs/cgroup/kubepods/pod*/; do
    [ -d "$d" ] || continue
    cpu=$(sed -n 's/^usage_usec //p' "$d/cpu.stat" 2>/dev/null)
    [ -n "$cpu" ] || continue
    uid=$(basename "$d")
    mc=$(cat "$d/memory.current" 2>/dev/null)
    inf=$(sed -n 's/^inactive_file //p' "$d/memory.stat" 2>/dev/null)
    echo "$uid $cpu ${mc:-0} ${inf:-0}"
  done
  echo "==="
  sleep $INT
done`

// Profile streams resource snapshots of every pod on the cluster's node by
// reading cgroup accounting directly. It writes one JSON Snapshot per line to
// emit, every interval, until duration elapses (duration <= 0 streams until
// ctx is cancelled). It is language- and workload-agnostic.
func Profile(ctx context.Context, cfg *config.Config, interval, duration time.Duration, emit io.Writer) error {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	secs := strconv.FormatFloat(interval.Seconds(), 'f', -1, 64)
	script := fmt.Sprintf(profileScript, secs)

	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	cmd := runtime.Command("exec", cfg.ServerName, "sh", "-c", script)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("profile: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("profile: starting node sampler: %w", err)
	}
	logger.Debug("profile: sampling node ", cfg.ServerName, " every ", secs, "s")

	// Kill the node sampler when the context ends (duration/interrupt).
	go func() {
		<-ctx.Done()
		_ = cmd.Process.Kill()
	}()

	enc := json.NewEncoder(emit)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	pods := make(map[string]PodSample)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "===" {
			snap := Snapshot{TimeMillis: time.Now().UnixMilli(), Pods: pods}
			if err := enc.Encode(snap); err != nil {
				return fmt.Errorf("profile: encoding snapshot: %w", err)
			}
			pods = make(map[string]PodSample)
			continue
		}
		uid, s, ok := parsePodLine(line)
		if ok {
			pods[uid] = s
		}
	}
	// A killed process surfaces as a scanner/Wait error; that is the normal
	// way Profile stops, so treat a cancelled context as clean completion.
	werr := cmd.Wait()
	if ctx.Err() != nil {
		return nil
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("profile: reading node stream: %w", err)
	}
	return werr
}

// parsePodLine parses "uid cpu_usec mem_current inactive_file" into a sample.
// The uid is the cgroup directory name; it is normalised to the Kubernetes
// pod UID (strip the "pod" prefix and any ".slice" suffix, map _ back to -).
func parsePodLine(line string) (string, PodSample, bool) {
	f := strings.Fields(line)
	if len(f) != 4 {
		return "", PodSample{}, false
	}
	uid := strings.TrimSuffix(strings.TrimPrefix(f[0], "pod"), ".slice")
	uid = strings.ReplaceAll(uid, "_", "-")
	cpu, e1 := strconv.ParseInt(f[1], 10, 64)
	mc, e2 := strconv.ParseInt(f[2], 10, 64)
	inf, e3 := strconv.ParseInt(f[3], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return "", PodSample{}, false
	}
	ws := mc - inf
	if ws < 0 {
		ws = 0
	}
	return uid, PodSample{CPUUsec: cpu, MemWorkingSet: ws, MemCurrent: mc}, true
}
