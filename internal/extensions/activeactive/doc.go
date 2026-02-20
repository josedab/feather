// Package activeactive provides CRDT-based active-active replication
// with gossip protocol for multi-datacenter feature store deployments.
//
// It supports configurable conflict resolution strategies (last-writer-wins,
// highest-version, or custom) and peer-to-peer synchronization via a
// gossip protocol, enabling low-latency reads from any datacenter while
// maintaining eventual consistency across all replicas.
package activeactive
