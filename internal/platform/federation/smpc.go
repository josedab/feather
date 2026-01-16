package federation

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// SMPCProtocol represents a secure multi-party computation protocol.
type SMPCProtocol string

// SMPCProtocol constants.
const (
	ProtocolSecretSharing    SMPCProtocol = "secret_sharing"
	ProtocolGarbledCircuits  SMPCProtocol = "garbled_circuits"
	ProtocolObliviousTransfer SMPCProtocol = "oblivious_transfer"
	ProtocolHomomorphic      SMPCProtocol = "homomorphic"
)

// Party represents a participant in an SMPC computation.
type Party struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	PublicKey []byte    `json:"public_key"`
	Status    string    `json:"status"`
	JoinedAt  time.Time `json:"joined_at"`
}

// SecretShare represents one share of a secret-shared value.
type SecretShare struct {
	PartyID     string `json:"party_id"`
	ShareIndex  int    `json:"share_index"`
	Value       []byte `json:"value"`
	Threshold   int    `json:"threshold"`
	TotalShares int    `json:"total_shares"`
}

// ComputeRequest represents a request for secure computation.
type ComputeRequest struct {
	ID          string                   `json:"id"`
	Protocol    SMPCProtocol             `json:"protocol"`
	Operation   string                   `json:"operation"`
	Parties     []string                 `json:"parties"`
	InputShares map[string]*SecretShare  `json:"input_shares"`
	CreatedAt   time.Time                `json:"created_at"`
}

// ComputeResult represents the result of a secure computation.
type ComputeResult struct {
	RequestID        string    `json:"request_id"`
	Operation        string    `json:"operation"`
	Result           float64   `json:"result"`
	ParticipantCount int       `json:"participant_count"`
	VerificationHash string    `json:"verification_hash"`
	ComputedAt       time.Time `json:"computed_at"`
	DurationMs       int64     `json:"duration_ms"`
}

// SMPCConfig holds configuration for the SMPC engine.
type SMPCConfig struct {
	DefaultProtocol    SMPCProtocol `json:"default_protocol"`
	MinParties         int          `json:"min_parties"`
	DefaultThreshold   int          `json:"default_threshold"`
	MaxComputeTimeMs   int64        `json:"max_compute_time_ms"`
	EnableVerification bool         `json:"enable_verification"`
}

// DefaultSMPCConfig returns sensible defaults for SMPC configuration.
func DefaultSMPCConfig() SMPCConfig {
	return SMPCConfig{
		DefaultProtocol:    ProtocolSecretSharing,
		MinParties:         3,
		DefaultThreshold:   2,
		MaxComputeTimeMs:   30000,
		EnableVerification: true,
	}
}

// SMPCStats holds statistics for the SMPC engine.
type SMPCStats struct {
	TotalComputations      int     `json:"total_computations"`
	SuccessfulComputations int     `json:"successful_computations"`
	FailedComputations     int     `json:"failed_computations"`
	AvgDurationMs          float64 `json:"avg_duration_ms"`
	PartiesRegistered      int     `json:"parties_registered"`
}

// SMPCEngine manages secure multi-party computation.
type SMPCEngine struct {
	mu       sync.RWMutex
	config   SMPCConfig
	parties  map[string]*Party
	requests map[string]*ComputeRequest
	results  map[string]*ComputeResult
}

// NewSMPCEngine creates a new SMPC engine with the given configuration.
func NewSMPCEngine(cfg SMPCConfig) *SMPCEngine {
	return &SMPCEngine{
		config:   cfg,
		parties:  make(map[string]*Party),
		requests: make(map[string]*ComputeRequest),
		results:  make(map[string]*ComputeResult),
	}
}

// RegisterParty registers a new party for SMPC computations.
func (e *SMPCEngine) RegisterParty(party *Party) error {
	if party == nil {
		return fmt.Errorf("party is nil")
	}
	if party.ID == "" {
		return fmt.Errorf("party ID is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.parties[party.ID]; exists {
		return fmt.Errorf("party %q already registered", party.ID)
	}

	if party.Status == "" {
		party.Status = "active"
	}
	if party.JoinedAt.IsZero() {
		party.JoinedAt = time.Now()
	}

	e.parties[party.ID] = party
	return nil
}

// RemoveParty removes a party from the engine.
func (e *SMPCEngine) RemoveParty(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.parties[id]; !exists {
		return fmt.Errorf("party %q not found", id)
	}

	delete(e.parties, id)
	return nil
}

// GetParty returns a party by ID.
func (e *SMPCEngine) GetParty(id string) (*Party, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	party, exists := e.parties[id]
	if !exists {
		return nil, fmt.Errorf("party %q not found", id)
	}
	return party, nil
}

// ListParties returns all registered parties.
func (e *SMPCEngine) ListParties() []*Party {
	e.mu.RLock()
	defer e.mu.RUnlock()

	parties := make([]*Party, 0, len(e.parties))
	for _, p := range e.parties {
		parties = append(parties, p)
	}
	return parties
}

// prime is a large prime used for finite field arithmetic in secret sharing.
var prime = new(big.Int).SetInt64(2147483647) // Mersenne prime 2^31 - 1

// CreateShares splits a value into shares using simplified Shamir's Secret Sharing.
func (e *SMPCEngine) CreateShares(value float64, threshold, totalShares int) ([]*SecretShare, error) {
	if threshold < 1 {
		return nil, fmt.Errorf("threshold must be at least 1")
	}
	if totalShares < threshold {
		return nil, fmt.Errorf("total shares must be >= threshold")
	}

	// Convert value to big.Int (scaled by 1e6 for precision).
	scaled := int64(value * 1e6)
	secret := new(big.Int).SetInt64(scaled)
	secret.Mod(secret, prime)

	// Generate random polynomial coefficients: a_0 = secret, a_1..a_{t-1} random.
	coeffs := make([]*big.Int, threshold)
	coeffs[0] = secret
	for i := 1; i < threshold; i++ {
		c, err := rand.Int(rand.Reader, prime)
		if err != nil {
			return nil, fmt.Errorf("generating coefficient: %w", err)
		}
		coeffs[i] = c
	}

	// Evaluate polynomial at points 1..totalShares.
	shares := make([]*SecretShare, totalShares)
	for i := 0; i < totalShares; i++ {
		x := new(big.Int).SetInt64(int64(i + 1))
		y := evaluatePolynomial(coeffs, x, prime)

		shares[i] = &SecretShare{
			ShareIndex:  i + 1,
			Value:       y.Bytes(),
			Threshold:   threshold,
			TotalShares: totalShares,
		}
	}

	return shares, nil
}

// evaluatePolynomial evaluates a polynomial at point x in a finite field.
func evaluatePolynomial(coeffs []*big.Int, x, mod *big.Int) *big.Int {
	result := new(big.Int).Set(coeffs[0])
	xPow := new(big.Int).SetInt64(1)

	for i := 1; i < len(coeffs); i++ {
		xPow.Mul(xPow, x)
		xPow.Mod(xPow, mod)

		term := new(big.Int).Mul(coeffs[i], xPow)
		term.Mod(term, mod)

		result.Add(result, term)
		result.Mod(result, mod)
	}

	return result
}

// ReconstructSecret reconstructs a secret from threshold shares using Lagrange interpolation.
func (e *SMPCEngine) ReconstructSecret(shares []*SecretShare) (float64, error) {
	if len(shares) == 0 {
		return 0, fmt.Errorf("no shares provided")
	}

	threshold := shares[0].Threshold
	if len(shares) < threshold {
		return 0, fmt.Errorf("need at least %d shares, got %d", threshold, len(shares))
	}

	// Use the first `threshold` shares for reconstruction.
	used := shares[:threshold]

	secret := new(big.Int)
	for i, si := range used {
		xi := new(big.Int).SetInt64(int64(si.ShareIndex))
		yi := new(big.Int).SetBytes(si.Value)

		// Compute Lagrange basis polynomial l_i(0).
		num := new(big.Int).SetInt64(1)
		den := new(big.Int).SetInt64(1)

		for j, sj := range used {
			if i == j {
				continue
			}
			xj := new(big.Int).SetInt64(int64(sj.ShareIndex))

			// num *= -xj  (mod prime)
			negXj := new(big.Int).Neg(xj)
			negXj.Mod(negXj, prime)
			num.Mul(num, negXj)
			num.Mod(num, prime)

			// den *= (xi - xj) (mod prime)
			diff := new(big.Int).Sub(xi, xj)
			diff.Mod(diff, prime)
			den.Mul(den, diff)
			den.Mod(den, prime)
		}

		// Compute den^{-1} mod prime.
		denInv := new(big.Int).ModInverse(den, prime)
		if denInv == nil {
			return 0, fmt.Errorf("modular inverse does not exist")
		}

		// l_i(0) = num / den
		li := new(big.Int).Mul(num, denInv)
		li.Mod(li, prime)

		// Accumulate yi * l_i(0).
		term := new(big.Int).Mul(yi, li)
		term.Mod(term, prime)

		secret.Add(secret, term)
		secret.Mod(secret, prime)
	}

	// Convert back from scaled integer.
	return float64(secret.Int64()) / 1e6, nil
}

// SubmitCompute submits a compute request for later execution.
func (e *SMPCEngine) SubmitCompute(req *ComputeRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}
	if req.ID == "" {
		return fmt.Errorf("request ID is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.requests[req.ID]; exists {
		return fmt.Errorf("request %q already exists", req.ID)
	}

	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.Protocol == "" {
		req.Protocol = e.config.DefaultProtocol
	}

	e.requests[req.ID] = req
	return nil
}

// ExecuteCompute executes a previously submitted compute request.
func (e *SMPCEngine) ExecuteCompute(requestID string) (*ComputeResult, error) {
	e.mu.Lock()
	req, exists := e.requests[requestID]
	if !exists {
		e.mu.Unlock()
		return nil, fmt.Errorf("request %q not found", requestID)
	}
	e.mu.Unlock()

	start := time.Now()

	// Collect input values from shares.
	values := make([]float64, 0, len(req.InputShares))
	for _, share := range req.InputShares {
		val := new(big.Int).SetBytes(share.Value)
		values = append(values, float64(val.Int64())/1e6)
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("no input values for computation")
	}

	var result float64
	switch req.Operation {
	case "sum":
		for _, v := range values {
			result += v
		}
	case "avg":
		for _, v := range values {
			result += v
		}
		result /= float64(len(values))
	case "count":
		result = float64(len(values))
	case "min":
		result = values[0]
		for _, v := range values[1:] {
			if v < result {
				result = v
			}
		}
	case "max":
		result = values[0]
		for _, v := range values[1:] {
			if v > result {
				result = v
			}
		}
	default:
		return nil, fmt.Errorf("unsupported operation: %s", req.Operation)
	}

	duration := time.Since(start)

	var verificationHash string
	if e.config.EnableVerification {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%.6f", requestID, req.Operation, result)))
		verificationHash = fmt.Sprintf("%x", h)
	}

	cr := &ComputeResult{
		RequestID:        requestID,
		Operation:        req.Operation,
		Result:           result,
		ParticipantCount: len(req.Parties),
		VerificationHash: verificationHash,
		ComputedAt:       time.Now(),
		DurationMs:       duration.Milliseconds(),
	}

	e.mu.Lock()
	e.results[requestID] = cr
	e.mu.Unlock()

	return cr, nil
}

// GetResult returns a compute result by request ID.
func (e *SMPCEngine) GetResult(requestID string) (*ComputeResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result, exists := e.results[requestID]
	if !exists {
		return nil, fmt.Errorf("result for request %q not found", requestID)
	}
	return result, nil
}

// ListResults returns all compute results.
func (e *SMPCEngine) ListResults() []*ComputeResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	results := make([]*ComputeResult, 0, len(e.results))
	for _, r := range e.results {
		results = append(results, r)
	}
	return results
}

// Stats returns engine statistics.
func (e *SMPCEngine) Stats() *SMPCStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &SMPCStats{
		TotalComputations: len(e.results),
		PartiesRegistered: len(e.parties),
	}

	var totalDuration int64
	for _, r := range e.results {
		stats.SuccessfulComputations++
		totalDuration += r.DurationMs
	}

	stats.FailedComputations = stats.TotalComputations - stats.SuccessfulComputations
	if stats.SuccessfulComputations > 0 {
		stats.AvgDurationMs = float64(totalDuration) / float64(stats.SuccessfulComputations)
	}

	return stats
}
