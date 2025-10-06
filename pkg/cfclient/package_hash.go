package cfclient

import (
	"crypto/sha256"
	"encoding/hex"
)

func ShortHash(guid string) string {
	hash := sha256.Sum256([]byte(guid))
	hexHash := hex.EncodeToString(hash[:])
	return hexHash[:7]
}
