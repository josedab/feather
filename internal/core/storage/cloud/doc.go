// Package cloud provides cloud-native storage backends for Feather.
//
// It implements the StorageBackend interface for various cloud providers,
// enabling Feather to use managed storage services instead of local BadgerDB.
//
// Supported backends:
//   - DynamoDB: AWS DynamoDB with optional DAX caching
//   - S3: AWS S3 for historical/cold data
//   - GCS: Google Cloud Storage
//   - Bigtable: Google Cloud Bigtable
//
// # Usage
//
//	backend, err := cloud.NewDynamoDBBackend(cloud.DynamoDBConfig{
//	    Region:    "us-east-1",
//	    TableName: "feather-features",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	store := storage.NewStoreWithBackend(backend)
package cloud
