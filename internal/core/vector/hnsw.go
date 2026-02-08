// Package vector provides vector similarity search using HNSW algorithm.
package vector

import (
	"container/heap"
	"crypto/rand"
	"encoding/binary"
	"math"
	"sync"
)

// Object pools for search operations to reduce GC pressure.
var (
	// Pool for visited maps used during search
	visitedMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]bool, 128)
		},
	}

	// Pool for distItem slices used in results
	distItemSlicePool = sync.Pool{
		New: func() interface{} {
			s := make([]distItem, 0, 64)
			return &s
		},
	}

	// Pool for random buffers used in level selection
	levelRandBufPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, 8)
			return &buf
		},
	}
)

// HNSW implements Hierarchical Navigable Small World graph for ANN search.
// This is the same algorithm used by Pinecone, Weaviate, and Milvus.
type HNSW struct {
	mu sync.RWMutex

	// Configuration
	dim         int     // Vector dimension
	m           int     // Max connections per node
	mMax        int     // Max connections per node at level 0
	efConstruct int     // Size of dynamic candidate list during construction
	ml          float64 // Level multiplier

	// Graph structure
	nodes      map[string]*node
	entryPoint string
	maxLevel   int

	// Distance function
	distFunc DistanceFunc
}

// node represents a point in the HNSW graph.
type node struct {
	id      string
	vector  []float32
	level   int
	friends [][]string // friends[level] = list of friend IDs at that level
}

// DistanceFunc computes distance between two vectors.
type DistanceFunc func(a, b []float32) float32

// DistanceType represents the type of distance metric.
type DistanceType string

const (
	// DistanceCosine uses cosine distance.
	DistanceCosine DistanceType = "cosine"
	// DistanceEuclidean uses Euclidean distance.
	DistanceEuclidean DistanceType = "euclidean"
	// DistanceDotProduct uses dot product distance.
	DistanceDotProduct DistanceType = "dot_product"
)

// HNSWConfig configures the HNSW index.
type HNSWConfig struct {
	Dim          int          // Vector dimension (required)
	M            int          // Max connections per node (default: 16)
	EfConstruct  int          // Construction search size (default: 200)
	DistanceType DistanceType // Distance metric (default: cosine)
}

// NewHNSW creates a new HNSW index.
func NewHNSW(config HNSWConfig) *HNSW {
	if config.M == 0 {
		config.M = 16
	}
	if config.EfConstruct == 0 {
		config.EfConstruct = 200
	}
	if config.DistanceType == "" {
		config.DistanceType = DistanceCosine
	}

	var distFunc DistanceFunc
	switch config.DistanceType {
	case DistanceCosine:
		distFunc = cosineDistance
	case DistanceEuclidean:
		distFunc = euclideanDistance
	case DistanceDotProduct:
		distFunc = dotProductDistance
	default:
		distFunc = cosineDistance
	}

	return &HNSW{
		dim:         config.Dim,
		m:           config.M,
		mMax:        config.M * 2,
		efConstruct: config.EfConstruct,
		ml:          1.0 / math.Log(float64(config.M)),
		nodes:       make(map[string]*node),
		maxLevel:    -1,
		distFunc:    distFunc,
	}
}

// Insert adds a vector to the index.
func (h *HNSW) Insert(id string, vector []float32) error {
	if len(vector) != h.dim {
		return ErrDimensionMismatch
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if already exists
	if _, exists := h.nodes[id]; exists {
		// Update existing
		h.nodes[id].vector = vector
		return nil
	}

	// Determine level for new node
	level := h.randomLevel()

	// Create node
	n := &node{
		id:      id,
		vector:  vector,
		level:   level,
		friends: make([][]string, level+1),
	}
	for i := range n.friends {
		n.friends[i] = make([]string, 0)
	}

	h.nodes[id] = n

	// If first node, set as entry point
	if h.entryPoint == "" {
		h.entryPoint = id
		h.maxLevel = level
		return nil
	}

	// Find entry point for insertion
	ep := h.entryPoint

	// Traverse from top level to node's level + 1
	for l := h.maxLevel; l > level; l-- {
		ep = h.searchLayerClosest(vector, ep, l)
	}

	// Insert at each level from node's level down to 0
	for l := minValue(level, h.maxLevel); l >= 0; l-- {
		neighbors := h.searchLayer(vector, ep, h.efConstruct, l)

		// Select M best neighbors
		maxConn := h.m
		if l == 0 {
			maxConn = h.mMax
		}
		selectedNeighbors := h.selectNeighbors(vector, neighbors, maxConn)

		// Add bidirectional connections
		n.friends[l] = selectedNeighbors
		for _, neighborID := range selectedNeighbors {
			neighbor := h.nodes[neighborID]
			neighbor.friends[l] = append(neighbor.friends[l], id)

			// Prune if necessary
			if len(neighbor.friends[l]) > maxConn {
				neighbor.friends[l] = h.selectNeighbors(neighbor.vector, neighbor.friends[l], maxConn)
			}
		}

		if len(neighbors) > 0 {
			ep = neighbors[0]
		}
	}

	// Update entry point if new node has higher level
	if level > h.maxLevel {
		h.entryPoint = id
		h.maxLevel = level
	}

	return nil
}

// Search finds the k nearest neighbors to the query vector.
func (h *HNSW) Search(query []float32, k int, ef int) ([]SearchResult, error) {
	if len(query) != h.dim {
		return nil, ErrDimensionMismatch
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.entryPoint == "" {
		return nil, nil
	}

	if ef == 0 {
		ef = maxValue(k, 10)
	}

	// Find entry point by traversing from top
	ep := h.entryPoint
	for l := h.maxLevel; l > 0; l-- {
		ep = h.searchLayerClosest(query, ep, l)
	}

	// Search layer 0 with ef candidates
	candidates := h.searchLayer(query, ep, ef, 0)

	// Return top k
	results := make([]SearchResult, 0, minValue(k, len(candidates)))
	for i := 0; i < k && i < len(candidates); i++ {
		n := h.nodes[candidates[i]]
		results = append(results, SearchResult{
			ID:       candidates[i],
			Distance: h.distFunc(query, n.vector),
			Vector:   n.vector,
		})
	}

	return results, nil
}

// Delete removes a vector from the index.
func (h *HNSW) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	n, exists := h.nodes[id]
	if !exists {
		return nil
	}

	// Remove connections from neighbors
	for l := 0; l <= n.level; l++ {
		for _, friendID := range n.friends[l] {
			friend := h.nodes[friendID]
			if friend == nil {
				continue
			}
			// Remove id from friend's list
			newFriends := make([]string, 0, len(friend.friends[l]))
			for _, fid := range friend.friends[l] {
				if fid != id {
					newFriends = append(newFriends, fid)
				}
			}
			friend.friends[l] = newFriends
		}
	}

	delete(h.nodes, id)

	// Update entry point if needed
	if h.entryPoint == id {
		h.entryPoint = ""
		h.maxLevel = -1
		for nid, nn := range h.nodes {
			if nn.level > h.maxLevel {
				h.maxLevel = nn.level
				h.entryPoint = nid
			}
		}
	}

	return nil
}

// Get retrieves a vector by ID.
func (h *HNSW) Get(id string) ([]float32, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	n, exists := h.nodes[id]
	if !exists {
		return nil, false
	}
	return n.vector, true
}

// Size returns the number of vectors in the index.
func (h *HNSW) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// searchLayerClosest finds the single closest node at a given level.
func (h *HNSW) searchLayerClosest(query []float32, ep string, level int) string {
	current := ep
	currentDist := h.distFunc(query, h.nodes[current].vector)

	for {
		changed := false
		node := h.nodes[current]
		if level >= len(node.friends) {
			break
		}

		for _, friendID := range node.friends[level] {
			friend := h.nodes[friendID]
			if friend == nil {
				continue
			}
			dist := h.distFunc(query, friend.vector)
			if dist < currentDist {
				current = friendID
				currentDist = dist
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	return current
}

// searchLayer performs a greedy search at a given level, returning ef closest nodes.
func (h *HNSW) searchLayer(query []float32, ep string, ef int, level int) []string {
	// Get visited map from pool
	visited, visitedOK := visitedMapPool.Get().(map[string]bool)
	if !visitedOK {
		visited = make(map[string]bool, 128)
	}
	// Clear the map for reuse
	for k := range visited {
		delete(visited, k)
	}
	defer visitedMapPool.Put(visited)

	visited[ep] = true

	candidates := &distHeap{}
	results := &distHeap{}

	epNode := h.nodes[ep]
	epDist := h.distFunc(query, epNode.vector)

	heap.Push(candidates, &distItem{id: ep, dist: epDist})
	heap.Push(results, &distItem{id: ep, dist: -epDist}) // negative for max-heap behavior

	for candidates.Len() > 0 {
		closest, itemOK := heap.Pop(candidates).(*distItem)
		if !itemOK {
			break
		}

		// Get furthest in results
		if results.Len() > 0 {
			furthest := (*results)[0]
			if closest.dist > -furthest.dist {
				break
			}
		}

		node := h.nodes[closest.id]
		if level >= len(node.friends) {
			continue
		}

		for _, friendID := range node.friends[level] {
			if visited[friendID] {
				continue
			}
			visited[friendID] = true

			friend := h.nodes[friendID]
			if friend == nil {
				continue
			}

			dist := h.distFunc(query, friend.vector)

			if results.Len() < ef {
				heap.Push(candidates, &distItem{id: friendID, dist: dist})
				heap.Push(results, &distItem{id: friendID, dist: -dist})
			} else if furthest := (*results)[0]; dist < -furthest.dist {
				heap.Push(candidates, &distItem{id: friendID, dist: dist})
				heap.Pop(results)
				heap.Push(results, &distItem{id: friendID, dist: -dist})
			}
		}
	}

	// Get result slices from pool
	resultListPtr, listOK := distItemSlicePool.Get().(*[]distItem)
	if !listOK {
		list := make([]distItem, 0, 64)
		resultListPtr = &list
	}
	resultList := (*resultListPtr)[:0]
	defer func() {
		*resultListPtr = resultList[:0]
		distItemSlicePool.Put(resultListPtr)
	}()

	// Extract results (sorted by distance)
	for results.Len() > 0 {
		item, itemOK := heap.Pop(results).(*distItem)
		if !itemOK {
			break
		}
		resultList = append(resultList, distItem{id: item.id, dist: -item.dist})
	}

	// Reverse to get ascending order (we popped in descending)
	for i, j := 0, len(resultList)-1; i < j; i, j = i+1, j-1 {
		resultList[i], resultList[j] = resultList[j], resultList[i]
	}

	// Build result IDs - this slice is returned so we can't pool it
	ids := make([]string, len(resultList))
	for i, item := range resultList {
		ids[i] = item.id
	}

	return ids
}

// selectNeighbors selects the best neighbors using simple heuristic.
func (h *HNSW) selectNeighbors(query []float32, candidates []string, m int) []string {
	if len(candidates) <= m {
		return candidates
	}

	// Sort by distance and take top m
	type scored struct {
		id   string
		dist float32
	}
	scoredCandidates := make([]scored, len(candidates))
	for i, id := range candidates {
		n := h.nodes[id]
		scoredCandidates[i] = scored{id: id, dist: h.distFunc(query, n.vector)}
	}

	// Simple selection sort for small m
	for i := 0; i < m; i++ {
		minIdx := i
		for j := i + 1; j < len(scoredCandidates); j++ {
			if scoredCandidates[j].dist < scoredCandidates[minIdx].dist {
				minIdx = j
			}
		}
		scoredCandidates[i], scoredCandidates[minIdx] = scoredCandidates[minIdx], scoredCandidates[i]
	}

	result := make([]string, m)
	for i := 0; i < m; i++ {
		result[i] = scoredCandidates[i].id
	}
	return result
}

func (h *HNSW) randomLevel() int {
	level := 0
	for h.randFloat64() < 0.5 && level < 16 {
		level++
	}
	return level
}

func (h *HNSW) randFloat64() float64 {
	bufPtr, ok := levelRandBufPool.Get().(*[]byte)
	if !ok {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			return 0.5
		}
		return float64(binary.LittleEndian.Uint64(buf)) / float64(^uint64(0))
	}
	buf := *bufPtr
	if _, err := rand.Read(buf); err != nil {
		levelRandBufPool.Put(bufPtr)
		return 0.5
	}
	levelRandBufPool.Put(bufPtr)
	return float64(binary.LittleEndian.Uint64(buf)) / float64(^uint64(0))
}

// SearchResult represents a search result.
type SearchResult struct {
	ID       string    `json:"id"`
	Distance float32   `json:"distance"`
	Vector   []float32 `json:"vector,omitempty"`
}

// Distance functions

func cosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1
	}
	similarity := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return 1 - similarity
}

func euclideanDistance(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return float32(math.Sqrt(float64(sum)))
}

func dotProductDistance(a, b []float32) float32 {
	var dot float32
	for i := range a {
		dot += a[i] * b[i]
	}
	// Convert to distance (higher dot product = smaller distance)
	return -dot
}

// Heap implementation for priority queue

type distItem struct {
	id   string
	dist float32
}

type distHeap []*distItem

func (h distHeap) Len() int           { return len(h) }
func (h distHeap) Less(i, j int) bool { return h[i].dist < h[j].dist }
func (h distHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *distHeap) Push(x interface{}) {
	item, ok := x.(*distItem)
	if !ok {
		return
	}
	*h = append(*h, item)
}

func (h *distHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func minValue(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxValue(a, b int) int {
	if a > b {
		return a
	}
	return b
}
