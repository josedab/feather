package schemaevolution

import "errors"

var (
	// ErrIncompatibleSchema is returned when a schema change breaks compatibility.
	ErrIncompatibleSchema = errors.New("incompatible schema change")

	// ErrMigrationNotFound is returned when a migration doesn't exist.
	ErrMigrationNotFound = errors.New("migration not found")

	// ErrMigrationInProgress is returned when another migration is running.
	ErrMigrationInProgress = errors.New("migration already in progress")

	// ErrSchemaNotFound is returned when a schema version doesn't exist.
	ErrSchemaNotFound = errors.New("schema not found")
)
