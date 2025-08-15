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
