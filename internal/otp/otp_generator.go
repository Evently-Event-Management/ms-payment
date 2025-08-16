package otp

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateOTP returns a 6-digit random code as a string
func GenerateOTP() (string, error) {
	max := big.NewInt(1000000) // 10^6
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
