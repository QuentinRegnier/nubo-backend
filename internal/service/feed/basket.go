package feed

import (
	"fmt"
	"math"
	"sync"
)

// ─────────────────────────────────────────────────────────────────────────────
// CONFIGURATION DES QUOTAS & ORIGINES
// ─────────────────────────────────────────────────────────────────────────────

// CandidateOrigin définit la provenance d'un post dans le panier pour l'A/B testing et les métriques.
type CandidateOrigin string

const (
	OriginSocial CandidateOrigin = "SOCIAL" // Issu des abonnements ou amis
	OriginTag    CandidateOrigin = "TAG"    // Issu des préférences thématiques de l'utilisateur
	OriginGlobal CandidateOrigin = "GLOBAL" // Issu des tendances pures (Sérendipité / Découverte)
)

// Quotas définit les règles de répartition et la taille cible du panier de candidats bruts.
type Quotas struct {
	MaxCandidates int     // Taille cible totale du panier (ex: 1000)
	SocialRatio   float64 // Proportion de posts issus du réseau d'abonnements/amis (ex: 0.3)
	TagRatio      float64 // Proportion de posts issus des affinités thématiques (ex: 0.5)
	GlobalRatio   float64 // Proportion de posts issus des tendances globales (ex: 0.2)
}

// Validate s'assure de l'exactitude mathématique et de la cohérence des quotas injectés.
func (fq *Quotas) Validate() error {
	if fq.MaxCandidates <= 0 {
		return fmt.Errorf("le nombre maximum de candidats doit être strictement positif")
	}

	// Tolérance aux imprécisions microscopiques d'arrondi des float
	sum := fq.SocialRatio + fq.TagRatio + fq.GlobalRatio
	if math.Abs(sum-1.0) > 1e-6 {
		return fmt.Errorf("la somme des ratios de distribution doit être strictement égale à 1.0 (actuellement: %f)", sum)
	}

	if fq.SocialRatio < 0 || fq.TagRatio < 0 || fq.GlobalRatio < 0 {
		return fmt.Errorf("les ratios de distribution ne peuvent pas être négatifs")
	}

	return nil
}

// GetQuotaSizes convertit les ratios en tailles absolues d'IDs à collecter.
// Sécurise le calcul pour éviter toute perte ou surplus d'unité lié aux arrondis de flottants.
func (fq *Quotas) GetQuotaSizes() (socialSize, tagSize, globalSize int) {
	socialSize = int(math.Round(float64(fq.MaxCandidates) * fq.SocialRatio))
	tagSize = int(math.Round(float64(fq.MaxCandidates) * fq.TagRatio))

	// Le dernier segment prend le reste exact pour garantir la stricte égalité avec MaxCandidates
	globalSize = fq.MaxCandidates - (socialSize + tagSize)
	return
}

// ─────────────────────────────────────────────────────────────────────────────
// LE PANIER DE CANDIDATS (BASKET)
// ─────────────────────────────────────────────────────────────────────────────

// BasketItem représente un post dans le panier avant son passage en "caisse" (MMR).
type BasketItem struct {
	PostID int64
	Origin CandidateOrigin
}

// CandidateBasket représente le "panier de courses" de l'agent magasinier.
type CandidateBasket struct {
	mu        sync.RWMutex       // Sécurité pour l'ajout asynchrone par les Goroutines du magasinier
	UniqueIDs map[int64]struct{} // Déduplication absolue en O(1)
	Items     []BasketItem       // La liste finale conservant les métadonnées de provenance
}

// NewCandidateBasket initialise un panier avec une capacité pré-allouée pour soulager le Garbage Collector.
func NewCandidateBasket(capacity int) *CandidateBasket {
	return &CandidateBasket{
		UniqueIDs: make(map[int64]struct{}, capacity),
		Items:     make([]BasketItem, 0, capacity),
	}
}

// Add tente d'ajouter un candidat au panier. Retourne true si ajouté, false si déjà présent.
func (b *CandidateBasket) Add(postID int64, origin CandidateOrigin) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.UniqueIDs[postID]; exists {
		return false
	}

	b.UniqueIDs[postID] = struct{}{}
	b.Items = append(b.Items, BasketItem{
		PostID: postID,
		Origin: origin,
	})

	return true
}

// Size retourne le nombre actuel de candidats uniques dans le panier.
func (b *CandidateBasket) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.Items)
}
