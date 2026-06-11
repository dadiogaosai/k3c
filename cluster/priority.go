package cluster

import (
	"strconv"

	"github.com/philipparndt/go-logger"

	"k3c/config"
)

// applyCPUPriority clamps the cluster's virtual machine processes to the
// macOS "utility" QoS band, so interactive applications always win CPU
// contention (video calls stay smooth through boot storms and reconcile
// bursts) while the cluster freely uses idle cores. Disabled with
// cpuPriority: normal.
func applyCPUPriority(cfg *config.Config) {
	if cfg.CPUPriority == "normal" {
		return
	}
	for _, name := range []string{cfg.ServerName, cfg.RegistryName} {
		pid := vzProcessPID(name)
		if pid == 0 {
			continue
		}
		if out, err := runOut("taskpolicy", "-c", "utility", "-p", strconv.Itoa(pid)); err != nil {
			logger.Debug("cpu priority for " + name + ": " + out)
		}
	}
}
