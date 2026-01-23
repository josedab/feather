package anomalydetect

import "errors"

var (
	// ErrFeatureNotMonitored is returned when checking a feature that has not been registered.
	ErrFeatureNotMonitored = errors.New("feature is not monitored")

	// ErrQuarantined is returned when a feature has been quarantined due to anomalies.
	ErrQuarantined = errors.New("feature is quarantined")
)
