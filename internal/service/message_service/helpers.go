package message_service

import (
	"regexp"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// Compilation globale de la regex pour des performances optimales (O(N) sans recompilation).
// On cherche le motif @{id}, ex: @{123456}
var mentionRegex = regexp.MustCompile(`@{([0-9]+)}`)

// ExtractMentions parse le contenu d'un message et retourne une liste
// dédupliquée des IDs d'utilisateurs mentionnés.
func ExtractMentions(content string) []int64 {
	// FindAllStringSubmatch retourne les correspondances et les groupes de capture
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var ids []int64
	for _, match := range matches {
		// match[0] est la chaîne complète "@{123}", match[1] est le groupe capturé "123"
		if len(match) == 2 {
			if id, err := strconv.ParseInt(match[1], 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
	}

	// Déduplication via ton utilitaire existant pour éviter qu'un spam
	// @{123} @{123} @{123} ne génère plusieurs évaluations.
	return pkg.SliceUniqueInt64(ids)
}
