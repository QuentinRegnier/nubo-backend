package algorithm_service

import (
	"context"
	"math/rand"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/nubo_error"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg/logger"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ############################################################################
// # PILIER 3 : APPROXIMATION PAR LOCALITÉ (LSH)
// ############################################################################

// LSHEngine encapsule la matrice de projection aléatoire P ∈ R^{b×N}
type LSHEngine struct {
	projectionMatrix []float32
	hashBits         int // Par défaut : 32
	vectorDimension  int // Par défaut : 224
}

// DefaultLSHEngine est l'instance singleton partagée par tous les workers Go.
var DefaultLSHEngine *LSHEngine

func init() {
	DefaultLSHEngine = NewLSHEngine(variables.TDDLSHSeed)
}

// NewLSHEngine crée un LSHEngine avec une matrice de projection aléatoire.
func NewLSHEngine(seed int64) *LSHEngine {
	bitsCount := variables.TDDLSHBits
	dimensionSize := variables.VectorDimTotal

	randomGen := rand.New(rand.NewSource(seed))
	matrixSize := bitsCount * dimensionSize
	generatedMatrix := make([]float32, matrixSize)

	for i := range generatedMatrix {
		generatedMatrix[i] = float32(randomGen.NormFloat64())
	}

	return &LSHEngine{
		projectionMatrix: generatedMatrix,
		hashBits:         bitsCount,
		vectorDimension:  dimensionSize,
	}
}

// ############################################################################
// # CALCUL DU HASH ET OPÉRATIONS LSH
// ############################################################################

// ComputeHash calcule le hash LSH d'un vecteur v ∈ R^N.
func (engine *LSHEngine) ComputeHash(vector []float32) uint32 {
	if len(vector) < engine.vectorDimension {
		return 0
	}

	var computedHash uint32

	for bitIndex := 0; bitIndex < engine.hashBits; bitIndex++ {
		baseIndex := bitIndex * engine.vectorDimension

		var dotProduct float32
		matrixRow := engine.projectionMatrix[baseIndex : baseIndex+engine.vectorDimension]

		for j, projectionValue := range matrixRow {
			dotProduct += projectionValue * vector[j]
		}

		if dotProduct > 0 {
			computedHash |= 1 << uint(bitIndex)
		}
	}

	return computedHash
}

// NeighborHashes retourne les hashes voisins à distance de Hamming ≤ 1.
func (engine *LSHEngine) NeighborHashes(targetHash uint32) []uint32 {
	neighborList := make([]uint32, 0, engine.hashBits+1)
	neighborList = append(neighborList, targetHash) // Bucket exact

	for bitIndex := 0; bitIndex < engine.hashBits; bitIndex++ {
		neighborList = append(neighborList, targetHash^(1<<uint(bitIndex)))
	}

	return neighborList
}

// ############################################################################
// # INTERACTIONS REDIS (GESTION DES BUCKETS LSH)
// ############################################################################

// StoreLSHBucket enregistre un post dans son bucket LSH Redis.
func StoreLSHBucket(ctx context.Context, postID int64, hashValue uint32) error {
	memberIDString := strconv.FormatInt(postID, 10)

	if err := redis.LSHBuckets.SAdd(ctx, hashValue, memberIDString); err != nil {
		return nubo_error.NewInternal()
	}

	_ = redis.LSHBuckets.RefreshTTL(ctx, hashValue)
	return nil
}

// GetLSHCandidateIDs récupère l'ensemble des IDs présents dans les buckets voisins.
func GetLSHCandidateIDs(ctx context.Context, targetHash uint32) (map[int64]bool, error) {
	neighborHashes := DefaultLSHEngine.NeighborHashes(targetHash)
	candidateSet := make(map[int64]bool, 200)

	for _, hashValue := range neighborHashes {
		memberIDs, err := redis.LSHBuckets.SMembers(ctx, hashValue)
		if err != nil {
			logger.Log.Warn().Err(err).Uint32("bucket", hashValue).Msg("Lookup LSH bucket ignoré")
			continue
		}

		for _, memberStr := range memberIDs {
			if parsedID, errParse := strconv.ParseInt(memberStr, 10, 64); errParse == nil {
				candidateSet[parsedID] = true
			}
		}
	}

	return candidateSet, nil
}

// RemoveLSHBucket retire un post de son bucket LSH.
func RemoveLSHBucket(ctx context.Context, postID int64, hashValue uint32) error {
	return redis.LSHBuckets.SRem(ctx, hashValue, strconv.FormatInt(postID, 10))
}

// PurgePostVectors supprime le vecteur d'engagement du post et le retire de son bucket LSH.
func PurgePostVectors(ctx context.Context, postID int64) error {
	var payload ContentVectorPayload

	if err := redis.ContentVectors.GetObject(ctx, postID, &payload); err == nil {
		_ = RemoveLSHBucket(ctx, postID, payload.LSHHash)
	}

	return redis.ContentVectors.DeleteObject(ctx, postID)
}
