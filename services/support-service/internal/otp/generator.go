package otp

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Generate retourne un code OTP de 6 chiffres généré via crypto/rand.
func Generate() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
