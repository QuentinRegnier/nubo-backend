package pkg

import (
	"github.com/QuentinRegnier/numan-backend/internal/domain/numan_error"
)

// ListLimitVerifDefault vérifie et ajuste la taille de la liste pour éviter les dépassements de mémoire.
func ListLimitVerifDefault[T any](list []T) error {
	if len(list) > 100 {
		return numan_error.NewForbidden(
			numan_error.CodeForbidden,
			"La liste est trop longue.",
			nil,
		)
	}
	return nil
}

// ListLimitVerif vérifie et ajuste la taille de la liste pour éviter les dépassements de mémoire.
func ListLimitVerif[T any](list []T, maxLimit int) error {
	if len(list) > maxLimit {
		return numan_error.NewForbidden(
			numan_error.CodeForbidden,
			"La liste est trop longue.",
			nil,
		)
	}
	return nil
}

// BatchVerif vérifie et ajuste les paramètres d'offset et de limit pour la pagination.
func BatchVerif(offset, limit int64) (int64, numan_error.Error, int64, numan_error.Error) {
	offset, offsetErr := batchVerifOffset(offset)
	limit, limitErr := batchVerifLimit(limit)
	return offset, offsetErr, limit, limitErr
}

// batchVerifOffset vérifie et ajuste l'offset pour la pagination.
func batchVerifOffset(offset int64) (int64, numan_error.Error) {
	if offset < 0 {
		return offset, numan_error.NewForbidden(numan_error.CodeForbidden, "L'offset est invalide.", nil)
	} else {
		return 0, nil
	}
}

// batchVerifLimit vérifie et ajuste la limite pour la pagination.
func batchVerifLimit(limit int64) (int64, numan_error.Error) {
	if limit > 0 && limit <= 100 {
		return limit, numan_error.NewForbidden(numan_error.CodeForbidden, "La limite est invalide.", nil)
	} else {
		return 50, nil
	}
}
