package types

import (
	"crypto/sha256"
	"encoding/hex"
)

func Hash256(data string) string {
	hasher := sha256.New()
	// Write the data to the hasher
	hasher.Write([]byte(data))
	// Get the hash result as a byte slice
	hashBytes := hasher.Sum(nil)
	// Convert the byte slice to a hex string
	return hex.EncodeToString(hashBytes)
}

type HasHash interface {
	HashID() string
}

// Helper function to extract IDs from links
func GetHashIDs[T HasHash](x []T) []string {
	ids := make([]string, len(x))
	for i, y := range x {
		ids[i] = y.HashID()
	}
	return ids
}
