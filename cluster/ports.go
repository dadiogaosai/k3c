package cluster

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"k3c/config"
)

// Clusters publish their API, ingress, and registry on private per-cluster
// host ports (allocated at create, persisted in the cluster state dir).
// The public ingress (:443 SNI gateway) and registry ports are owned by the
// always-running k3c daemon, which routes to the *active* cluster — so
// multiple clusters can coexist and switching (pause/resume) is instant,
// without fighting over sockets held by frozen VMs.

type clusterPorts struct {
	API      string `yaml:"api"`
	Ingress  string `yaml:"ingress"`
	Registry string `yaml:"registry"`
}

func portsFile(cfg *config.Config) string {
	return filepath.Join(cfg.RunDir(), "ports.yaml")
}

func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	_, port, err := net.SplitHostPort(l.Addr().String())
	return port, err
}

// ensurePorts allocates (or loads) the cluster's private host ports and
// applies them to the config.
func ensurePorts(cfg *config.Config) error {
	if err := loadPorts(cfg); err == nil {
		return nil
	}
	var ports clusterPorts
	var err error
	if ports.API, err = freePort(); err != nil {
		return err
	}
	if ports.Ingress, err = freePort(); err != nil {
		return err
	}
	if ports.Registry, err = freePort(); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.RunDir(), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(ports)
	if err != nil {
		return err
	}
	if err := os.WriteFile(portsFile(cfg), data, 0o644); err != nil {
		return err
	}
	applyPorts(cfg, ports)
	return nil
}

// loadPorts applies persisted per-cluster ports to the config; clusters
// created before this feature keep the legacy shared public ports.
func loadPorts(cfg *config.Config) error {
	data, err := os.ReadFile(portsFile(cfg))
	if err != nil {
		return err
	}
	var ports clusterPorts
	if err := yaml.Unmarshal(data, &ports); err != nil {
		return err
	}
	applyPorts(cfg, ports)
	return nil
}

func applyPorts(cfg *config.Config, ports clusterPorts) {
	if ports.API != "" {
		cfg.APIPortInternal = ports.API
	}
	if ports.Ingress != "" {
		cfg.IngressPortInternal = ports.Ingress
	}
	if ports.Registry != "" {
		cfg.RegistryPortInternal = ports.Registry
	}
}

// activeState names the cluster the daemon routes public traffic to. When
// Sidecar is set the docker sidecar is the active target: it owns every host
// port both it and the active cluster publish (contested ports). The Cluster
// fields are kept either way, so non-contested cluster routing (registry, the
// :443 egress path) still works while the sidecar holds the contested ports.
type activeState struct {
	Cluster        string   `yaml:"cluster"`
	IngressPort    string   `yaml:"ingressPort"`
	RegistryPort   string   `yaml:"registryPort"`
	IngressDomains []string `yaml:"ingressDomains"`
	Sidecar        bool     `yaml:"sidecar"`
}

func activeFile(cfg *config.Config) string {
	return filepath.Join(cfg.BaseDir, "active.yaml")
}

// ActiveClusterName returns the active cluster (the routing target most
// recently created, started, or resumed), or "" when none is recorded or
// it no longer exists.
func ActiveClusterName() string {
	dir := config.StateDir()
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "active.yaml"))
	if err != nil {
		return ""
	}
	var state activeState
	if err := yaml.Unmarshal(data, &state); err != nil || state.Cluster == "" {
		return ""
	}
	if !containerExists(state.Cluster+"-server", false) {
		return ""
	}
	return state.Cluster
}

// setActive routes the daemon's public ports to this cluster and makes the
// cluster the active target — Sidecar is cleared, so activating a cluster (incl.
// the implicit activate on start/resume) reclaims any contested ports the
// sidecar held.
func setActive(cfg *config.Config) error {
	return writeActive(cfg, activeState{
		Cluster:        cfg.Cluster,
		IngressPort:    cfg.IngressPortInternal,
		RegistryPort:   cfg.RegistryPortInternal,
		IngressDomains: cfg.IngressDomains,
	})
}

// setActiveSidecar makes the docker sidecar the active target: it now owns the
// contested host ports. The cluster routing fields are preserved, so the last
// active cluster stays the registry/kube fallback for non-contested traffic.
func setActiveSidecar(cfg *config.Config) error {
	state := readActive(cfg)
	state.Sidecar = true
	return writeActive(cfg, state)
}

func writeActive(cfg *config.Config, state activeState) error {
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.WriteFile(activeFile(cfg), data, 0o644); err != nil {
		return fmt.Errorf("recording active target: %w", err)
	}
	return nil
}

// readActive returns the routing target, falling back to the daemon's own
// config for legacy setups.
func readActive(cfg *config.Config) activeState {
	fallback := activeState{
		Cluster:        cfg.Cluster,
		IngressPort:    cfg.IngressPortInternal,
		RegistryPort:   cfg.RegistryPortInternal,
		IngressDomains: cfg.IngressDomains,
	}
	data, err := os.ReadFile(activeFile(cfg))
	if err != nil {
		return fallback
	}
	var state activeState
	if err := yaml.Unmarshal(data, &state); err != nil || state.IngressPort == "" {
		return fallback
	}
	return state
}
