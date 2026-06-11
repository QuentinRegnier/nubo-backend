package algorithm_service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// PILIER 3 — APPROXIMATION PAR LOCALITÉ (LSH — LOCALITY-SENSITIVE HASHING)
// TDD §4.5
// ============================================================================

// LSHEngine encapsule la matrice de projection aléatoire P ∈ R^{b×N}
// et expose les opérations de hachage vectoriel.
//
// TDD §4.5:
//
//	P ∈ R^{b×224}, b = 32 bits
//	Graine fixe pour la reproductibilité — distribuée à tous les workers Go.
//	LSH(v) = bin(sign(P·v)) ∈ {0,1}^32
type LSHEngine struct {
	// proj stocke P en ordre row-major: proj[i*dim+j] = P[i][j]
	// Dimension: TDDLSHBits × VectorDimTotal = 32 × 224 = 7168 float32
	proj []float32
	bits int // = TDDLSHBits = 32
	dim  int // = VectorDimTotal = 224
}

// DefaultLSHEngine est l'instance singleton partagée par tous les workers Go.
// Initialisé une seule fois au démarrage du package avec la graine TDD fixe.
var DefaultLSHEngine *LSHEngine

func init() {
	// TDD §4.5: "graine fixe pour la reproductibilité"
	DefaultLSHEngine = NewLSHEngine(variables.TDDLSHSeed)
}

// NewLSHEngine crée un LSHEngine avec une matrice de projection aléatoire.
//
// TDD §4.5:
//
//	P ∈ R^{b×N} — chaque entrée tirée d'une loi normale standard N(0,1)
//	Seed fixe → déterminisme total entre processus et redémarrages
//
// La matrice P est générée une seule fois (O(b·N) = O(7168) opérations).
func NewLSHEngine(seed int64) *LSHEngine {
	bits := variables.TDDLSHBits
	dim := variables.VectorDimTotal

	// Génération de P ∈ R^{b×N} — distribution normale standard N(0,1)
	// Graine fixe garantissant la cohérence entre tous les workers (TDD §4.5)
	rng := rand.New(rand.NewSource(seed))
	proj := make([]float32, bits*dim)

	// Remplissage row-major: P[i][j] = proj[i*dim+j] ~ N(0,1)
	for i := range proj {
		proj[i] = float32(rng.NormFloat64())
	}

	return &LSHEngine{
		proj: proj,
		bits: bits,
		dim:  dim,
	}
}

// ComputeHash calcule le hash LSH d'un vecteur v ∈ R^N.
//
// TDD §4.5:
//
//	LSH(v) = bin(sign(P·v)) ∈ {0,1}^b
//
// Algorithme:
//  1. Pour chaque rangée i de P: calculer dot_i = Σ_j P[i][j] · v[j]
//  2. Bit i = 1 si dot_i > 0, sinon 0
//
// Complexité: O(b·N) = O(32·224) = O(7168) multiplications SIMD.
//
// Propriété probabiliste (TDD §4.5):
//
//	Pr[LSH(u) = LSH(c_p)] = 1 - arccos(<û, ĉ_p>) / π
func (e *LSHEngine) ComputeHash(v []float32) uint32 {
	if len(v) < e.dim {
		return 0
	}

	var h uint32

	// Pour chaque bit i (0..b-1): calculer le produit scalaire de la rangée i avec v
	//
	// TDD §4.5: O(32·224) = O(7168) opérations
	// Boucle interne SIMD-friendly: le compilateur Go vectorise avec AVX2 sur x86-64
	for i := 0; i < e.bits; i++ {
		base := i * e.dim

		// dot_i = Σ_{j=0}^{223} P[i][j] · v[j]
		var dot float32
		row := e.proj[base : base+e.dim]
		for j, pij := range row {
			dot += pij * v[j]
		}

		// bit i ← I[dot_i > 0]
		if dot > 0 {
			h |= 1 << uint(i)
		}
	}
	return h
}

// NeighborHashes retourne les hashes voisins à distance de Hamming ≤ 1.
//
// TDD §4.5: "lookup de 32 buckets voisins"
//
// Retourne 33 hashes:
//   - hash exact (distance Hamming = 0): 1 bucket
//   - 32 hashes avec 1 bit flippé (distance Hamming = 1): 32 buckets
//
// Total: 33 lookups Redis couvrant les vecteurs les plus similaires.
func (e *LSHEngine) NeighborHashes(h uint32) []uint32 {
	// Pré-allocation: 1 exact + 32 voisins = 33 buckets (TDD §4.5)
	neighbors := make([]uint32, 0, e.bits+1)
	neighbors = append(neighbors, h) // Bucket exact

	// 32 voisins à distance de Hamming = 1 (flip du bit i)
	for i := 0; i < e.bits; i++ {
		neighbors = append(neighbors, h^(1<<uint(i)))
	}
	return neighbors
}

// StoreLSHBucket enregistre un post_service dans son bucket LSH Redis.
//
// TDD §4.5: "Redis la table lsh:bucket:{bit32_code} (type Set)"
// TTL = 7 jours (aligné sur le TTL du vecteur de contenu)
func StoreLSHBucket(ctx context.Context, postID int64, hash uint32) error {
	member := strconv.FormatInt(postID, 10)

	if err := redis.LSHBuckets.SAdd(ctx, hash, member); err != nil {
		return fmt.Errorf("sadd lsh bucket %d: %w", hash, err)
	}

	// Rafraîchissement du TTL encapsulé (le bucket est partagé entre plusieurs posts)
	_ = redis.LSHBuckets.RefreshTTL(ctx, hash)
	return nil
}

// GetLSHCandidateIDs récupère l'ensemble des post_service IDs présents dans les buckets
// voisins du hash donné (exact + 32 single-bit-flip neighbors).
//
// TDD §4.5:
//
//	"Récupère depuis Redis la table lsh:bucket:{bit32_code} contenant les post_ids
//	 avec un LSH identique ou à distance de Hamming ≤ 4 (via lookup de 32 buckets voisins)"
//
// Retourne un set (map[int64]bool) pour des lookups O(1) lors du filtrage.
func GetLSHCandidateIDs(ctx context.Context, hash uint32) (map[int64]bool, error) {
	neighbors := DefaultLSHEngine.NeighborHashes(hash)
	result := make(map[int64]bool, 200) // Capacité estimée: ~200 posts (TDD §4.5)

	for _, neighborHash := range neighbors {
		members, err := redis.LSHBuckets.SMembers(ctx, neighborHash)
		if err != nil {
			// Bucket manquant ou erreur Redis: ignorer silencieusement
			log.Printf("⚠️ [lsh] SMembers bucket %d: %v", neighborHash, err)
			continue
		}

		for _, m := range members {
			id, err := strconv.ParseInt(m, 10, 64)
			if err == nil {
				result[id] = true
			}
		}
	}

	return result, nil
}

// RemoveLSHBucket retire un post_service de son bucket LSH (utile lors de la suppression d'un post_service).
func RemoveLSHBucket(ctx context.Context, postID int64, hash uint32) error {
	return redis.LSHBuckets.SRem(ctx, hash, strconv.FormatInt(postID, 10))
}

// PurgePostVectors supprime le vecteur d'engagement du post et le retire de son bucket LSH
func PurgePostVectors(ctx context.Context, postID int64) error {
	// 1. Lecture via L1 Object Cache pour récupérer le LSHHash
	// ✅ CORRECTION BUG CRITIQUE : Utilise MsgPack et non plus un JSON Unmarshal obsolète
	var payload ContentVectorPayload
	if err := redis.ContentVectors.GetObject(ctx, postID, &payload); err == nil {
		// On retire le post de son bucket LSH
		_ = RemoveLSHBucket(ctx, postID, payload.LSHHash)
	}

	// 2. Purge définitive du vecteur via Collection
	return redis.ContentVectors.DeleteObject(ctx, postID)
}

// LSHConfidenceThreshold est le seuil de confiance utilisateur pour activer le LSH.
// TDD §4.5: "Pour les utilisateurs avec un historique d'interactions suffisant (confidence > 0.70)"
const LSHConfidenceThreshold = 0.70
