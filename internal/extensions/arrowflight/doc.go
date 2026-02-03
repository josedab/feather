// Package arrowflight provides zero-copy columnar data transport for feature serving
// and training data export using the Apache Arrow Flight protocol.
//
// Arrow Flight enables 10-100x throughput improvements over JSON/Protobuf serialization
// for batch feature reads by leveraging columnar memory layout and zero-copy transfers.
//
// # Architecture
//
// The Flight server runs alongside existing gRPC/HTTP servers and provides three
// primary RPC endpoints:
//
//   - DoGet: Batch feature retrieval in columnar Arrow format
//   - DoPut: Bulk feature ingestion from Arrow record batches
//   - DoExchange: Bidirectional streaming for interactive queries
//
// # Usage
//
//	server := arrowflight.NewServer(arrowflight.DefaultConfig())
//	server.SetStore(store)
//	// Flight endpoints are now available for high-throughput data transport
package arrowflight
