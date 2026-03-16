package index

// bkTree is a BK-tree for efficient fuzzy (edit-distance) term lookup.
// Instead of computing Levenshtein distance against every term in the dictionary,
// a BK-tree prunes the search space using the triangle inequality property of
// edit distance: if d(query, node) = k, then any term within distance n of the
// query must be at distance [k-n, k+n] from the node.
type bkTree struct {
	root *bkNode
	size int
}

type bkNode struct {
	term     string
	children map[int]*bkNode // distance -> child
}

// newBKTree creates a new empty BK-tree.
func newBKTree() *bkTree {
	return &bkTree{}
}

// buildBKTree builds a BK-tree from a sorted term list.
func buildBKTree(terms []string) *bkTree {
	tree := newBKTree()
	for _, term := range terms {
		tree.insert(term)
	}
	return tree
}

// insert adds a term to the BK-tree.
func (t *bkTree) insert(term string) {
	if t.root == nil {
		t.root = &bkNode{term: term, children: make(map[int]*bkNode)}
		t.size++
		return
	}
	node := t.root
	for {
		d := editDistance(node.term, term)
		if d == 0 {
			return // duplicate
		}
		child, ok := node.children[d]
		if !ok {
			node.children[d] = &bkNode{term: term, children: make(map[int]*bkNode)}
			t.size++
			return
		}
		node = child
	}
}

// search finds all terms within the given edit distance of the query.
func (t *bkTree) search(query string, maxDist int) []string {
	if t.root == nil {
		return nil
	}
	var results []string
	var stack []*bkNode
	stack = append(stack, t.root)
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		d := editDistance(node.term, query)
		if d <= maxDist {
			results = append(results, node.term)
		}
		lo := d - maxDist
		hi := d + maxDist
		for dist, child := range node.children {
			if dist >= lo && dist <= hi {
				stack = append(stack, child)
			}
		}
	}
	return results
}

// editDistance computes the Levenshtein edit distance between two strings.
func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}
