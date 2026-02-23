// Package airflow provides integration with Apache Airflow for orchestrating
// feature pipeline DAGs.
//
// It supports creating DAG operators for feature computation, scheduling
// freshness checks, and managing feature pipeline dependencies within
// Airflow workflows. Configure via [Config] with Airflow URL and DAG prefix.
//
// Usage:
//
//	provider := airflow.NewProvider(airflow.Config{
//	    AirflowURL:             "http://localhost:8080",
//	    DAGPrefix:              "feather_",
//	    FreshnessCheckInterval: 5 * time.Minute,
//	})
//	provider.CreateDAG(ctx, operators)
package airflow
