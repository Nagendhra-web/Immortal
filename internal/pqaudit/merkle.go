package pqaudit

import (
	"crypto/sha256"
	"errors"
)

// MerkleRoot computes the SHA-256 binary Merkle root over the given leaf hashes.
// If the number of leaves is odd, the last leaf is duplicated before hashing.
// Returns nil for an empty slice.
func MerkleRoot(leafHashes [][]byte) []byte {
	if len(leafHashes) == 0 {
		return nil
	}
	layer := make([][]byte, len(leafHashes))
	for i, h := range leafHashes {
		cp := make([]byte, len(h))
		copy(cp, h)
		layer[i] = cp
	}
	for len(layer) > 1 {
		layer = merkleLayer(layer)
	}
	return layer[0]
}

// merkleLayer reduces one layer of the tree to its parent layer.
func merkleLayer(nodes [][]byte) [][]byte {
	if len(nodes)%2 != 0 {
		nodes = append(nodes, nodes[len(nodes)-1])
	}
	out := make([][]byte, len(nodes)/2)
	for i := 0; i < len(nodes); i += 2 {
		h := sha256.New()
		h.Write(nodes[i])
		h.Write(nodes[i+1])
		out[i/2] = h.Sum(nil)
	}
	return out
}

// MerkleProof returns the sibling hashes needed to prove that leafHashes[index]
// is part of the tree. The proof slice is ordered leaf-to-root.
func MerkleProof(leafHashes [][]byte, index int) ([][]byte, error) {
	if len(leafHashes) == 0 {
		return nil, errors.New("pqaudit: empty leaf set")
	}
	if index < 0 || index >= len(leafHashes) {
		return nil, errors.New("pqaudit: index out of range")
	}

	layer := make([][]byte, len(leafHashes))
	for i, h := range leafHashes {
		cp := make([]byte, len(h))
		copy(cp, h)
		layer[i] = cp
	}

	var proof [][]byte
	idx := index
	for len(layer) > 1 {
		if len(layer)%2 != 0 {
			layer = append(layer, layer[len(layer)-1])
		}
		sibling := idx ^ 1 // XOR with 1 flips between left/right sibling
		proof = append(proof, layer[sibling])
		// build parent layer
		layer = merkleLayer(layer)
		idx /= 2
	}
	return proof, nil
}

// VerifyMerkleProof checks that leaf at position index (in a tree of arbitrary
// size) hashes up to root using the provided proof sibling list.
func VerifyMerkleProof(leaf []byte, proof [][]byte, index int, root []byte) bool {
	current := make([]byte, len(leaf))
	copy(current, leaf)

	for _, sibling := range proof {
		h := sha256.New()
		if index%2 == 0 {
			h.Write(current)
			h.Write(sibling)
		} else {
			h.Write(sibling)
			h.Write(current)
		}
		current = h.Sum(nil)
		index /= 2
	}

	if len(root) != len(current) {
		return false
	}
	for i := range root {
		if root[i] != current[i] {
			return false
		}
	}
	return true
}
