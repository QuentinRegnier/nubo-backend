package feed_service

import (
	"context"
	"strings"
	"unicode"

	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
	"github.com/kljensen/snowball"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// ============================================================================
// NORMALISATION ET CANONICALISATION DES HASHTAGS — TDD §3.3
// ============================================================================

// NormalizeHashtag applique la normalisation lexicale (TDD §3.3 — Étape 1):
//
//	hashtag_normalized = lower(trim(transliterate(hashtag_raw)))
//
// Opérations:
//  1. Suppression des caractères non-alphanumériques en bordure
//  2. Translittération accentués → ASCII (NFD + suppression diacritiques Mn + NFC)
//  3. Conversion en minuscules
func NormalizeHashtag(raw string) string {
	// Étape 1a: trim des caractères non-alphanumériques en bordure
	trimmed := strings.TrimFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if trimmed == "" {
		return ""
	}
	// Étape 1b: translittération accentués → ASCII
	transliterated := transliterateToASCII(trimmed)
	// Étape 1c: conversion en minuscules
	return strings.ToLower(transliterated)
}

// transliterateToASCII convertit les caractères accentués vers leurs équivalents ASCII.
//
// TDD §3.3: "conversion des caractères accentués via golang.org/x/text/unicode/norm"
// Exemples: "ée" → "ee", "ç" → "c", "Naturisme" → "Naturisme"
//
// Méthode: décomposition NFD → suppression diacritiques (catégorie Unicode Mn) → NFC.
func transliterateToASCII(s string) string {
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	result, _, _ := transform.String(t, s)
	return result
}

// StemHashtag applique un stemming simplifié pour la canonicalisation des hashtags.
//
// TDD §3.3: "algorithme de Snowball stemming" pour vérifier la même racine morphologique.
func StemHashtag(normalized string) string {
	stemmed, err := snowball.Stem(normalized, "french", true)
	if err != nil {
		return normalized
	}
	return stemmed
}

// GetTagFromKeyword cherche le slug officiel (canonique) d'un hashtag.
func GetTagFromKeyword(ctx context.Context, input string) (string, bool) {
	cleanInput := NormalizeHashtag(input)
	if cleanInput == "" {
		return "", false
	}

	// 1. Vérification dans le mapping dynamique Redis (Fautes de frappe corrigées)
	canonSlug, err := redisgo.Rdb.HGet(ctx, variables.RedisKeyHashtagCanonMap, cleanInput).Result()
	if err == nil && canonSlug != "" {
		return canonSlug, true
	}

	// 2. Si inconnu, le mot propre devient son propre tag (Nouveau Tag Communautaire)
	// On l'ajoute silencieusement au SET des tags actifs pour que le Cron de nuit l'analyse.
	_ = redisgo.Rdb.SAdd(ctx, variables.RedisKeyActiveTagsSet, cleanInput).Err()

	return cleanInput, true
}
