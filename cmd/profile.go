package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"k3c/cluster"
)

var (
	profileInterval time.Duration
	profileDuration time.Duration
)

// profileCmd streams exact per-pod CPU and memory accounting read straight
// from the node's cgroup hierarchy, as JSON lines on stdout. It is the
// language-agnostic measurement primitive other tools build on (e.g. the
// veHub cli correlates these samples with pod readiness to compute exact
// CPU-until-ready and idle-CPU figures).
var profileCmd = &cobra.Command{
	Use:   "profile [NAME]",
	Short: "Stream exact per-pod CPU/memory accounting from the node cgroups",
	Long: `Stream exact per-pod resource accounting for a running cluster.

Unlike "kubectl top" (which reads cAdvisor stats refreshed only every ~10s),
profile reads cgroup v2 accounting directly on the node: cpu.stat usage_usec
(the kernel's own cumulative CPU billing) and the memory working set. Each
sampling tick is written to stdout as one JSON object:

  {"t_ms":<unix-ms>,"pods":{"<pod-uid>":{"cpu_usec":N,"mem_ws":N,"mem_current":N}}}

cpu_usec is cumulative since the pod started, so a consumer derives CPU rate
from the delta between two ticks, and CPU-until-ready from the tick nearest a
pod's Ready transition.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := loadConfigDefault(args)
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return cluster.Profile(ctx, cfg, profileInterval, profileDuration, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.Flags().DurationVar(&profileInterval, "interval", 500*time.Millisecond,
		"sampling interval (e.g. 250ms, 1s)")
	profileCmd.Flags().DurationVar(&profileDuration, "duration", 0,
		"stop after this long (0 = until interrupted)")
}
