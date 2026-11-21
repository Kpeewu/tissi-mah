package password

import (
	"crypto/rand"
	"math/big"
)

const (
	tempPasswordLength = 16
	lowerChars         = "abcdefghijklmnopqrstuvwxyz"
	upperChars         = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars         = "0123456789"
	specialChars       = "!@#$%^&*()-_=+[]{}"
)

// GenerateTemporary génère un mot de passe provisoire de 16 caractères contenant
// au moins une minuscule, une majuscule, un chiffre et un caractère spécial.
func GenerateTemporary() (string, error) {
	classes := []string{lowerChars, upperChars, digitChars, specialChars}
	all := lowerChars + upperChars + digitChars + specialChars

	out := make([]byte, tempPasswordLength)

	for i, class := range classes {
		c, err := pickFrom(class)
		if err != nil {
			return "", err
		}
		out[i] = c
	}
	for i := len(classes); i < tempPasswordLength; i++ {
		c, err := pickFrom(all)
		if err != nil {
			return "", err
		}
		out[i] = c
	}

	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

func pickFrom(s string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
	if err != nil {
		return 0, err
	}
	return s[n.Int64()], nil
}

func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		b[i], b[j.Int64()] = b[j.Int64()], b[i]
	}
	return nil
}
