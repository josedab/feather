// Package consistencyvalidator provides automated continuous validation that
// features served online match offline training data. It uses statistical
// divergence testing with configurable alerts and root-cause analysis.
//
// Key components:
//   - Validator: Continuously compares online and offline feature distributions
//   - Report: Contains divergence metrics and root-cause attribution
package consistencyvalidator
