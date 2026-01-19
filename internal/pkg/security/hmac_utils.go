package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// CheckHMAC vérifie la signature (utilisable par Middleware et Handler)
func CheckHMAC(stringToSign string, secret string, signatureToCheck string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	computedSig := hex.EncodeToString(h.Sum(nil))

	// Comparaison sécurisée (Constant Time)
	return hmac.Equal([]byte(computedSig), []byte(signatureToCheck))
}

// BuildStringToSign standardise la création de la chaîne (Method|Path|Ts|Body)
func BuildStringToSign(method, path, timestamp, body string) string {
	return fmt.Sprintf("%s|%s|%s|%s", method, path, timestamp, body)
}
