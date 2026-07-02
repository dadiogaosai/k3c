## ADDED Requirements

### Requirement: Memory policy settings

The config SHALL support `cluster.memoryPolicy` with values `auto` (default:
the runtime sizes the VMs' memory balloons continuously, returning unused
memory to the host) and `off` (VM memory is managed manually), and
`cluster.memoryHeadroom` (a size like `1500M`) for the memory kept available
to a guest above its workload, defaulting to the runtime's built-in headroom.
`cluster.autoReclaim` SHALL remain as the fallback interval for container
builds without runtime memory-policy support and SHALL be ignored otherwise.

#### Scenario: Opting out of automatic memory management

- **WHEN** the user sets `cluster.memoryPolicy: off` and creates a cluster
- **THEN** the VMs run without the runtime's balloon controller and keep
  their configured memory resident once touched
