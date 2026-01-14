// Package consensus provides a simplified Raft-like consensus implementation
// for metadata replication across cluster nodes. It builds on top of the
// cluster primitives (gossip membership, consistent hashing) to provide
// leader election, log replication, and shard assignment coordination.
package consensus
