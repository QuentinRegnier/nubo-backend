package security

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
)

// CheckHMAC vérifie la signature (utilisable par Middleware et Handler)
func CheckHMAC(stringToSign string, secret string, signatureToCheck string) bool {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	computedSig := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("🔐 CheckHMAC: stringToSign=%s, secret=%s, computedSig=%s, signatureToCheck=%s\n", stringToSign, secret, computedSig, signatureToCheck)

	// Comparaison sécurisée (Constant Time)
	return hmac.Equal([]byte(computedSig), []byte(signatureToCheck))
}

// BuildStringToSign standardise la création de la chaîne (Method|Path|Ts|Body)
func BuildStringToSign(method, path, timestamp, body string) string {
	return fmt.Sprintf("%s|%s|%s|%s", method, path, timestamp, body)
}

// GetBodyToSign extrait le contenu "utile" à signer selon le Content-Type
func GetBodyToSign(req *http.Request, bodyBytes []byte) string {
	contentType := req.Header.Get("Content-Type")

	// Si c'est du Multipart, on extrait le champ "data"
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// 1. Récupérer le "boundary" depuis le header
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil || params["boundary"] == "" {
			return string(bodyBytes) // Fallback si malformé
		}

		// 2. Créer un lecteur multipart sur les bytes (pour ne pas consommer le vrai body)
		reader := multipart.NewReader(bytes.NewReader(bodyBytes), params["boundary"])

		// 3. Parcourir les parts pour trouver "data"
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			// Si on trouve le champ "data", on lit son contenu et on le retourne
			if part.FormName() == "data" {
				buf := new(bytes.Buffer)
				if _, err := buf.ReadFrom(part); err != nil {
					return ""
				}
				return buf.String()
			}
		}
		// Si pas de champ data trouvé, on retourne vide (ou tout le body selon ta politique)
		return ""
	}

	// Sinon (JSON raw), on retourne tout le body
	return string(bodyBytes)
}

// =========================================================================
// ⚠️ ATTENTION : FONCTION DE SIGNATURE INTERNE (MICRO-SERVICES / URLS)
// =========================================================================
// Cette fonction sert EXCLUSIVEMENT à générer des signatures statiques
// pour des communications internes (ex: URL signée S3 vers le micro-service
// de tatouage/Watermark).
//
// Elle NE DOIT PAS être confondue ou utilisée avec la logique de sécurité
// dynamique (Ratchet / Sessions / WebSockets) de l'API publique qui est
// gérée spécifiquement dans "api/middleware/hmac.go".
// =========================================================================

// GenerateHMAC crée une signature SHA256 standardisée pour un payload donné.
func GenerateHMAC(payload string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}
