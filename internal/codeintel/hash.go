package codeintel

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// ChunkID returns a deterministic SHA-256 identifier for a chunk.
func ChunkID(filePath string, chunkType ChunkType, name string, lineStart int) string {
	input := filePath + string(chunkType) + name + strconv.Itoa(lineStart)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// ContentHash returns a deterministic SHA-256 hash of a chunk body.
func ContentHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
