package cloudstorage

import "errors"

var (
	// ErrObjectNotFound is returned when the requested object does not exist.
	ErrObjectNotFound = errors.New("object not found")

	// ErrBucketNotFound is returned when the requested bucket does not exist.
	ErrBucketNotFound = errors.New("bucket not found")

	// ErrStorageUnavailable is returned when the storage backend is unavailable.
	ErrStorageUnavailable = errors.New("storage unavailable")

	// ErrProviderNotSupported is returned when the storage provider is not supported.
	ErrProviderNotSupported = errors.New("provider not supported")
)
