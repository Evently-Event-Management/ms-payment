package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

func GenerateID() string {
	timestamp := time.Now().Unix()
	randomNum, _ := rand.Int(rand.Reader, big.NewInt(999999))
	return fmt.Sprintf("pay_%d_%06d", timestamp, randomNum.Int64())
}

func GenerateTransactionID() string {
	timestamp := time.Now().Unix()
	randomNum, _ := rand.Int(rand.Reader, big.NewInt(999999999))
	return fmt.Sprintf("txn_%d_%09d", timestamp, randomNum.Int64())
}

// GenerateUUID creates a random UUID v4
func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return GenerateID()
	}

	// Set version to 4 (random)
	uuid[6] = (uuid[6] & 0x0F) | 0x40
	// Set variant to RFC4122
	uuid[8] = (uuid[8] & 0x3F) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
