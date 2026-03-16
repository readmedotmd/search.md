package index

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
)

// HNSW implements Hierarchical Navigable Small World graphs for approximate
// nearest neighbor search. This is a pure-Go implementation that provides
// O(log N) query time instead of O(N) brute-force scan.
//
// Reference: Malkov & Yashunin, "Efficient and robust approximate nearest
// neighbor search using Hierarchical Navigable Small World graphs", 2018.
type HNSW struct {
	mu         sync.RWMutex
	nodes      map[string]*hnswNode // docID -> node
	entryPoint string               // docID of entry point
	maxLevel   int                  // current max level in the graph
	ml         float64              // level generation factor (1/ln(M))
	efConst    int                  // construction ef (beam width)
	M          int                  // max connections per layer
	M0         int                  // max connections at layer 0
	dims       int                  // vector dimensions
	rng        *rand.Rand
}

type hnswNode struct {
	id      string
	vector  []float32
	level   int
	friends [][]string // friends[layer] = list of neighbor docIDs
}

// NewHNSW creates a new HNSW index.
func NewHNSW(dims int) *HNSW {
	M := 16
	return &HNSW{
		nodes:   make(map[string]*hnswNode),
		ml:      1.0 / math.Log(float64(M)),
		efConst: 200,
		M:       M,
		M0:      M * 2,
		dims:    dims,
		rng:     rand.New(rand.NewSource(42)),
	}
}

func (h *HNSW) randomLevel() int {
	return int(-math.Log(h.rng.Float64()) * h.ml)
}

// Insert adds a vector to the HNSW graph.
func (h *HNSW) Insert(docID string, vec []float32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove old node if re-indexing.
	if old, exists := h.nodes[docID]; exists {
		h.removeNode(old)
	}

	level := h.randomLevel()
	node := &hnswNode{
		id:      docID,
		vector:  vec,
		level:   level,
		friends: make([][]string, level+1),
	}
	h.nodes[docID] = node

	if h.entryPoint == "" {
		h.entryPoint = docID
		h.maxLevel = level
		return
	}

	ep := h.entryPoint
	// Traverse from top level down to the node's level + 1.
	for l := h.maxLevel; l > level; l-- {
		ep = h.greedyClosest(vec, ep, l)
	}

	// Insert at each layer from min(level, maxLevel) down to 0.
	for l := min2(level, h.maxLevel); l >= 0; l-- {
		neighbors := h.searchLayer(vec, ep, h.efConst, l)
		maxConn := h.M
		if l == 0 {
			maxConn = h.M0
		}
		// Select M closest neighbors.
		if len(neighbors) > maxConn {
			neighbors = neighbors[:maxConn]
		}
		node.friends[l] = make([]string, len(neighbors))
		for i, n := range neighbors {
			node.friends[l][i] = n.id
			// Add bidirectional connection.
			if peer, ok := h.nodes[n.id]; ok {
				if l < len(peer.friends) {
					peer.friends[l] = append(peer.friends[l], docID)
					// Prune if over capacity.
					if len(peer.friends[l]) > maxConn {
						peer.friends[l] = h.pruneConnections(peer.vector, peer.friends[l], maxConn, l)
					}
				}
			}
		}
		if len(neighbors) > 0 {
			ep = neighbors[0].id
		}
	}

	if level > h.maxLevel {
		h.maxLevel = level
		h.entryPoint = docID
	}
}

// Delete removes a vector from the HNSW graph.
func (h *HNSW) Delete(docID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if node, exists := h.nodes[docID]; exists {
		h.removeNode(node)
		if h.entryPoint == docID {
			// Pick a new entry point.
			h.entryPoint = ""
			h.maxLevel = 0
			for id, n := range h.nodes {
				if n.level > h.maxLevel || h.entryPoint == "" {
					h.entryPoint = id
					h.maxLevel = n.level
				}
			}
		}
	}
}

func (h *HNSW) removeNode(node *hnswNode) {
	// Remove from all neighbors' friend lists.
	for l := 0; l < len(node.friends); l++ {
		for _, friendID := range node.friends[l] {
			if peer, ok := h.nodes[friendID]; ok && l < len(peer.friends) {
				filtered := peer.friends[l][:0]
				for _, fid := range peer.friends[l] {
					if fid != node.id {
						filtered = append(filtered, fid)
					}
				}
				peer.friends[l] = filtered
			}
		}
	}
	delete(h.nodes, node.id)
}

// Search finds the k nearest neighbors to the query vector.
// Returns docIDs and their cosine similarities (sorted descending).
func (h *HNSW) Search(query []float32, k, ef int) ([]string, []float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.entryPoint == "" || len(h.nodes) == 0 {
		return nil, nil
	}

	if ef < k {
		ef = k
	}

	ep := h.entryPoint
	for l := h.maxLevel; l > 0; l-- {
		ep = h.greedyClosest(query, ep, l)
	}

	candidates := h.searchLayer(query, ep, ef, 0)

	// Filter to positive similarity and take top-k.
	type result struct {
		id   string
		dist float64
	}
	var results []result
	for _, c := range candidates {
		sim := cosineSimF32(query, h.nodes[c.id].vector)
		if sim > 0 {
			results = append(results, result{c.id, sim})
		}
	}
	// Sort by similarity descending.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].dist > results[i].dist {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if len(results) > k {
		results = results[:k]
	}

	ids := make([]string, len(results))
	sims := make([]float64, len(results))
	for i, r := range results {
		ids[i] = r.id
		sims[i] = r.dist
	}
	return ids, sims
}

// Size returns the number of vectors in the index.
func (h *HNSW) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// greedyClosest finds the closest node to query at a given layer, starting from ep.
func (h *HNSW) greedyClosest(query []float32, ep string, level int) string {
	node := h.nodes[ep]
	if node == nil {
		return ep
	}
	bestDist := cosineDistF32(query, node.vector)
	changed := true
	for changed {
		changed = false
		if level >= len(node.friends) {
			break
		}
		for _, friendID := range node.friends[level] {
			friend := h.nodes[friendID]
			if friend == nil {
				continue
			}
			d := cosineDistF32(query, friend.vector)
			if d < bestDist {
				bestDist = d
				node = friend
				ep = friendID
				changed = true
			}
		}
	}
	return ep
}

type hnswCandidate struct {
	id   string
	dist float64
}

type candMinHeap []hnswCandidate

func (h candMinHeap) Len() int            { return len(h) }
func (h candMinHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h candMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *candMinHeap) Push(x interface{}) { *h = append(*h, x.(hnswCandidate)) }
func (h *candMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

type candMaxHeap []hnswCandidate

func (h candMaxHeap) Len() int            { return len(h) }
func (h candMaxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h candMaxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *candMaxHeap) Push(x interface{}) { *h = append(*h, x.(hnswCandidate)) }
func (h *candMaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// searchLayer performs beam search at a given layer.
func (h *HNSW) searchLayer(query []float32, ep string, ef, level int) []hnswCandidate {
	epNode := h.nodes[ep]
	if epNode == nil {
		return nil
	}
	d := cosineDistF32(query, epNode.vector)

	visited := map[string]bool{ep: true}
	candidates := &candMinHeap{{ep, d}}
	heap.Init(candidates)
	results := &candMaxHeap{{ep, d}}
	heap.Init(results)

	for candidates.Len() > 0 {
		c := heap.Pop(candidates).(hnswCandidate)
		// If the closest candidate is farther than the farthest result, stop.
		if results.Len() >= ef && c.dist > (*results)[0].dist {
			break
		}
		node := h.nodes[c.id]
		if node == nil || level >= len(node.friends) {
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
			fd := cosineDistF32(query, friend.vector)
			if results.Len() < ef || fd < (*results)[0].dist {
				heap.Push(candidates, hnswCandidate{friendID, fd})
				heap.Push(results, hnswCandidate{friendID, fd})
				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	// Drain results into a sorted slice.
	out := make([]hnswCandidate, results.Len())
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(results).(hnswCandidate)
	}
	return out
}

// pruneConnections selects the best maxConn neighbors for a node.
func (h *HNSW) pruneConnections(vec []float32, friendIDs []string, maxConn, level int) []string {
	type scored struct {
		id   string
		dist float64
	}
	var items []scored
	for _, fid := range friendIDs {
		if n, ok := h.nodes[fid]; ok {
			items = append(items, scored{fid, cosineDistF32(vec, n.vector)})
		}
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].dist < items[i].dist {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if len(items) > maxConn {
		items = items[:maxConn]
	}
	result := make([]string, len(items))
	for i, it := range items {
		result[i] = it.id
	}
	return result
}

// cosineDistF32 computes 1 - cosine_similarity for use as a distance metric.
func cosineDistF32(a, b []float32) float64 {
	return 1.0 - cosineSimF32(a, b)
}

// cosineSimF32 computes cosine similarity between two float32 vectors.
func cosineSimF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
