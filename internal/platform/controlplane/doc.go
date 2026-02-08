// Package controlplane provides a multi-cloud managed control plane for
// coordinating multiple Feather feature store instances across clouds and
// regions.
//
// # Architecture Overview
//
//	┌──────────────────────────────────────────────────┐
//	│                 Control Plane                     │
//	│                                                  │
//	│  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
//	│  │ Manager  │  │ Policies │  │  Replication   │  │
//	│  │          │──│          │──│    Manager     │  │
//	│  └──────────┘  └──────────┘  └───────────────┘  │
//	│       │                            │             │
//	│  ┌────┴────────────────────────────┴──────┐      │
//	│  │            Instance Registry           │      │
//	│  └────┬──────────┬──────────┬─────────────┘      │
//	│       │          │          │                     │
//	└───────┼──────────┼──────────┼────────────────────┘
//	        │          │          │
//	   ┌────┴───┐ ┌────┴───┐ ┌───┴────┐
//	   │ AWS    │ │  GCP   │ │ Azure  │
//	   │ Region │ │ Region │ │ Region │
//	   └────────┘ └────────┘ └────────┘
//
// # Core Components
//
// Manager is the central coordinator that tracks instances, regions, and
// policies. It maintains a registry of all running Feather instances and
// their health status.
//
// ReplicationManager handles cross-region data replication with configurable
// modes (sync, async, none) and conflict resolution policies.
//
// # Usage
//
//	mgr := controlplane.NewManager(controlplane.DefaultManagerConfig())
//
//	// Register a region
//	mgr.AddRegion(ctx, &controlplane.Region{
//	    Name:     "us-east-1",
//	    Provider: "aws",
//	    Primary:  true,
//	})
//
//	// Register an instance
//	mgr.RegisterInstance(ctx, &controlplane.Instance{
//	    Name:     "feather-prod-1",
//	    Region:   "us-east-1",
//	    Endpoint: "https://feather-1.example.com:8080",
//	})
//
//	// Check fleet status
//	status := mgr.GetFleetStatus(ctx)
package controlplane
