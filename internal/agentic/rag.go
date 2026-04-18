package agentic

import (
	"hash/fnv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Fingerprint is a 64-bit SimHash.
type Fingerprint uint64

// Hamming returns the number of bit positions that differ between two fingerprints.
func Hamming(a, b Fingerprint) int {
	x := uint64(a ^ b)
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// SimHash produces a 64-bit SimHash fingerprint over tokens in text.
// Tokenization: split on non-alphanumeric runes, lowercase, skip tokens shorter than 2 chars.
// Weight per token is its occurrence count.
func SimHash(text string) Fingerprint {
	features := make(map[string]int)
	for _, tok := range splitTokens(text) {
		features[tok]++
	}
	return SimHashWeighted(features)
}

// SimHashWeighted computes a SimHash from a pre-tokenized weighted feature map.
func SimHashWeighted(features map[string]int) Fingerprint {
	var vec [64]int
	h := fnv.New64a()
	for token, weight := range features {
		h.Reset()
		h.Write([]byte(token))
		bits := h.Sum64()
		for i := 0; i < 64; i++ {
			if bits&(1<<uint(i)) != 0 {
				vec[i] += weight
			} else {
				vec[i] -= weight
			}
		}
	}
	var fp Fingerprint
	for i := 0; i < 64; i++ {
		if vec[i] > 0 {
			fp |= Fingerprint(1) << uint(i)
		}
	}
	return fp
}

// splitTokens splits text on non-alphanumeric characters, lowercases each token,
// and discards tokens shorter than 2 characters.
func splitTokens(text string) []string {
	lower := strings.ToLower(text)
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len(t) >= 2 {
			out = append(out, t)
		}
	}
	return out
}

// semanticEntry extends MemoryEntry with a pre-computed SimHash fingerprint.
type semanticEntry struct {
	MemoryEntry
	fp Fingerprint
}

// SemanticMemory is a Memory enhancement that indexes each entry's SimHash.
// Unlike Memory.Recall (word-overlap), Recall returns entries ranked by Hamming
// distance — it catches "db connection timeout" ≈ "database connection timed out".
type SemanticMemory struct {
	mu       sync.RWMutex
	capacity int
	entries  []semanticEntry
	head     int
	size     int
}

// NewSemanticMemory returns a SemanticMemory with the given capacity (minimum 1).
func NewSemanticMemory(capacity int) *SemanticMemory {
	if capacity <= 0 {
		capacity = 100
	}
	return &SemanticMemory{
		capacity: capacity,
		entries:  make([]semanticEntry, capacity),
	}
}

// Record stores a new incident trace in the ring buffer, computing and caching its SimHash.
func (s *SemanticMemory) Record(incident Incident, trace *Trace, outcome Outcome) {
	s.mu.Lock()
	defer s.mu.Unlock()

	steps := make([]Step, len(trace.Steps))
	copy(steps, trace.Steps)

	fp := SimHash(incident.Message)
	s.entries[s.head] = semanticEntry{
		MemoryEntry: MemoryEntry{
			Timestamp: time.Now(),
			Incident:  incident,
			Trace:     steps,
			Outcome:   outcome,
		},
		fp: fp,
	}
	s.head = (s.head + 1) % s.capacity
	if s.size < s.capacity {
		s.size++
	}
}

// Recall returns up to k entries ranked by ascending Hamming distance to the query.
// Entries with equal Hamming distance are broken by recency (newer first).
func (s *SemanticMemory) Recall(query Incident, k int) []MemoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.size == 0 || k <= 0 {
		return nil
	}

	queryFP := SimHash(query.Message)

	type scoredSemantic struct {
		entry   MemoryEntry
		hamming int
		idx     int // insertion order (higher = newer)
	}

	results := make([]scoredSemantic, 0, s.size)
	for i := 0; i < s.size; i++ {
		pos := ((s.head - s.size + i) + s.capacity) % s.capacity
		e := s.entries[pos]
		results = append(results, scoredSemantic{
			entry:   e.MemoryEntry,
			hamming: Hamming(queryFP, e.fp),
			idx:     i,
		})
	}

	// Sort ascending by Hamming, ties broken by descending idx (newer first).
	for i := 1; i < len(results); i++ {
		key := results[i]
		j := i - 1
		for j >= 0 && (results[j].hamming > key.hamming ||
			(results[j].hamming == key.hamming && results[j].idx < key.idx)) {
			results[j+1] = results[j]
			j--
		}
		results[j+1] = key
	}

	if k > len(results) {
		k = len(results)
	}
	out := make([]MemoryEntry, k)
	for i := 0; i < k; i++ {
		out[i] = results[i].entry
	}
	return out
}

// Size returns the number of entries currently stored.
func (s *SemanticMemory) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.size
}
