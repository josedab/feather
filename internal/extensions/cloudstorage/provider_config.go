package cloudstorage

import (
	"fmt"
	"time"
)

// S3Config configures an S3-compatible storage backend.
type S3Config struct {
	Region          string        `json:"region" yaml:"region"`
	Bucket          string        `json:"bucket" yaml:"bucket"`
	Prefix          string        `json:"prefix" yaml:"prefix"`
	Endpoint        string        `json:"endpoint" yaml:"endpoint"`
	AccessKeyID     string        `json:"-" yaml:"-"`
	SecretAccessKey string        `json:"-" yaml:"-"`
	SessionToken    string        `json:"-" yaml:"-"`
	UsePathStyle    bool          `json:"use_path_style" yaml:"use_path_style"`
	SSEAlgorithm    string        `json:"sse_algorithm" yaml:"sse_algorithm"`
	SSEKMSKeyID     string        `json:"sse_kms_key_id" yaml:"sse_kms_key_id"`
	ConnectTimeout  time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
}

// DefaultS3Config returns sensible defaults for S3.
func DefaultS3Config() S3Config {
	return S3Config{
		Region:         "us-east-1",
		Bucket:         "feather-data",
		UsePathStyle:   false,
		SSEAlgorithm:   "AES256",
		ConnectTimeout: 10 * time.Second,
		MaxRetries:     3,
	}
}

// GCSConfig configures a Google Cloud Storage backend.
type GCSConfig struct {
	ProjectID       string        `json:"project_id" yaml:"project_id"`
	Bucket          string        `json:"bucket" yaml:"bucket"`
	Prefix          string        `json:"prefix" yaml:"prefix"`
	CredentialsFile string        `json:"-" yaml:"-"`
	CredentialsJSON string        `json:"-" yaml:"-"`
	Location        string        `json:"location" yaml:"location"`
	StorageClass    string        `json:"storage_class" yaml:"storage_class"`
	ConnectTimeout  time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
}

// DefaultGCSConfig returns sensible defaults for GCS.
func DefaultGCSConfig() GCSConfig {
	return GCSConfig{
		Bucket:         "feather-data",
		Location:       "US",
		StorageClass:   "STANDARD",
		ConnectTimeout: 10 * time.Second,
		MaxRetries:     3,
	}
}

// AzureBlobConfig configures an Azure Blob Storage backend.
type AzureBlobConfig struct {
	AccountName    string        `json:"account_name" yaml:"account_name"`
	AccountKey     string        `json:"-" yaml:"-"`
	ContainerName  string        `json:"container_name" yaml:"container_name"`
	Prefix         string        `json:"prefix" yaml:"prefix"`
	Endpoint       string        `json:"endpoint" yaml:"endpoint"`
	ConnectTimeout time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	MaxRetries     int           `json:"max_retries" yaml:"max_retries"`
}

// DefaultAzureBlobConfig returns sensible defaults for Azure Blob Storage.
func DefaultAzureBlobConfig() AzureBlobConfig {
	return AzureBlobConfig{
		ContainerName:  "feather-data",
		ConnectTimeout: 10 * time.Second,
		MaxRetries:     3,
	}
}

// NewBackendFromConfig creates a Backend from a provider-specific config.
// For non-local providers, this returns the local backend as a placeholder
// until the real SDK is integrated. The Backend interface is ready for
// drop-in replacement with aws-sdk-go-v2, cloud.google.com/go/storage, etc.
func NewBackendFromConfig(provider Provider, basePath string) (Backend, error) {
	switch provider {
	case ProviderLocal:
		return NewLocalBackend(basePath)
	case ProviderS3, ProviderGCS, ProviderAzure, ProviderMinIO:
		// Real cloud backends require SDK integration. Return local backend
		// with a provider-prefixed path as a development fallback.
		return NewLocalBackend(basePath + "/" + string(provider))
	default:
		return nil, fmt.Errorf("%w: %s", ErrProviderNotSupported, provider)
	}
}
