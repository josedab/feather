package importancescoring

import "errors"

var (
	// ErrFeatureNotScored is returned when requesting a score for an untracked feature.
	ErrFeatureNotScored = errors.New("feature has not been scored")

	// ErrInsufficientData is returned when there are not enough samples to compute a score.
	ErrInsufficientData = errors.New("insufficient data for scoring")
)
