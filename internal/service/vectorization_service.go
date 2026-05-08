package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/domain"
	redisgo "github.com/QuentinRegnier/nubo-backend/internal/infrastructure/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ============================================================================
// PILIER 3 — VECTORISATION DU CONTENU CÔTÉ SERVEUR
// TDD §4.1
// ============================================================================

// ContentVectorPayload est la structure sérialisée dans Redis sous content:vec:{post_id}.
//
// TDD §4.1: "mis en cache dans Redis (LFU Object Store) sous la clé content:vec:{post_id}"
// Les champs AuthorID et LSHHash sont inclus pour éviter des lookups supplémentaires
// lors du calcul de R(u,p) et du pré-filtrage LSH.
type ContentVectorPayload struct {
	V        []float32 `json:"v"`         // ĉ_p ∈ R^224 (normalisé L2)
	LSHHash  uint32    `json:"lsh"`       // Hash LSH pré-calculé pour le bucket §4.5
	AuthorID int64     `json:"author_id"` // Pour le calcul de B(u,p) §4.2
}

// ContentVectorOptions permet d'injecter les embeddings externes optionnels.
// Lorsque nil ou vide, les blocs correspondants sont initialisés à zéro
// (comportement dégradé gracieux jusqu'à disponibilité des données).
type ContentVectorOptions struct {
	// TagEmbeddings: E_h ∈ R^128 par hashtag canonique normalisé.
	// TDD §2.2 / §4.1: matrice d'embedding SVD distribuée aux workers.
	TagEmbeddings map[string][]float32

	// AuthorSocEmbed: g_author ∈ R^64 — embedding graphe social de l'auteur.
	// TDD §4.1: "identique au mécanisme de u^(soc) mais centré sur l'auteur du post"
	AuthorSocEmbed []float32
}

// ─────────────────────────────────────────────────────────────────────────────
// ENTRÉE PUBLIQUE — Compatibilité avec les workers existants
// ─────────────────────────────────────────────────────────────────────────────

// StoreContentVector calcule et stocke de manière asynchrone le vecteur ĉ_p du post.
//
// TDD §4.1:
//   - Calcul à la création du post
//   - Cache Redis: content:vec:{post_id}, TTL = 7 jours
//
// TDD §4.5:
//   - Met également à jour le bucket LSH: lsh:bucket:{hash}
//
// Signature préservée pour rétrocompatibilité avec les workers existants.
func StoreContentVector(ctx context.Context, post domain.PostRequest) {
	// Calcul du vecteur ĉ_p ∈ R^224 complet (nil opts = blocs SVD/soc à zéro)
	vec := computeContentVectorFull(post, nil)

	payload := ContentVectorPayload{
		V:        vec,
		LSHHash:  DefaultLSHEngine.ComputeHash(vec),
		AuthorID: post.UserID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("⚠️ [vect] sérialisation vecteur post %d: %v", post.ID, err)
		return
	}

	// TDD §4.1: TTL = 7 jours pour le cache LFU Object Store
	key := fmt.Sprintf(variables.RedisKeyContentVector, post.ID)
	if err := redisgo.Rdb.Set(ctx, key, data, 7*24*time.Hour).Err(); err != nil {
		log.Printf("⚠️ [vect] Redis SET content:vec:%d: %v", post.ID, err)
		return
	}

	// TDD §4.5: Mise à jour du bucket LSH pour le pré-filtrage
	if err := StoreLSHBucket(ctx, post.ID, payload.LSHHash); err != nil {
		log.Printf("⚠️ [vect] Redis LSH bucket post %d: %v", post.ID, err)
	}
}

// UpdatePostEngagementVector met à jour de manière asynchrone le bloc engagement
// c_p^(eng) suite à de nouveaux signaux d'engagement.
//
// TDD §4.1: "mises à jour suite à de nouveaux engagements effectuées de manière
// asynchrone par le worker de score"
func UpdatePostEngagementVector(ctx context.Context, post domain.PostRequest) {
	key := fmt.Sprintf(variables.RedisKeyContentVector, post.ID)

	// Lecture du payload existant
	rawData, err := redisgo.Rdb.Get(ctx, key).Bytes()
	if err != nil {
		log.Printf("⚠️ [vect-update] Redis GET content:vec:%d: %v", post.ID, err)
		return
	}
	var payload ContentVectorPayload
	if err := json.Unmarshal(rawData, &payload); err != nil || len(payload.V) != variables.VectorDimTotal {
		// Recalcul complet si le payload est corrompu
		StoreContentVector(ctx, post)
		return
	}

	// Mise à jour uniquement du bloc engagement [152:160)
	//
	// TDD §4.1: c_p^(eng) ∈ R^8 — miroir structurel de u^(eng)
	engBlock := payload.V[variables.VectorOffEng : variables.VectorOffEng+variables.VectorDimEng]
	computeEngBlock(post, engBlock)

	// Re-normalisation L2 après modification du bloc
	//
	// TDD §2.2: û = u / ||u||_2 — normalisation critique pour l'équivalence cosinus/dot
	NormalizeL2(payload.V)

	// Mise à jour du hash LSH après re-normalisation
	payload.LSHHash = DefaultLSHEngine.ComputeHash(payload.V)

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	redisgo.Rdb.Set(ctx, key, data, 7*24*time.Hour)
}

// ─────────────────────────────────────────────────────────────────────────────
// IMPLÉMENTATION INTERNE — Construction des 4 blocs du vecteur c_p
// ─────────────────────────────────────────────────────────────────────────────

// computeContentVectorFull construit le vecteur complet ĉ_p ∈ R^224 normalisé.
//
// TDD §4.1:
//
//	c_p = [c_p^(cat) | c_p^(temp) | c_p^(eng) | c_p^(soc)]
//	Normalisé: ĉ_p = c_p / ||c_p||_2
func computeContentVectorFull(post domain.PostRequest, opts *ContentVectorOptions) []float32 {
	// Allocation unique du vecteur (pré-initialisé à zéro)
	vec := make([]float32, variables.VectorDimTotal)

	// Extraire les sous-slices de chaque bloc (vues, pas de copies)
	catBlock := vec[variables.VectorOffCat : variables.VectorOffCat+variables.VectorDimCat]
	tempBlock := vec[variables.VectorOffTemp : variables.VectorOffTemp+variables.VectorDimTemp]
	engBlock := vec[variables.VectorOffEng : variables.VectorOffEng+variables.VectorDimEng]
	socBlock := vec[variables.VectorOffSoc : variables.VectorOffSoc+variables.VectorDimSoc]

	// ── Bloc 1 : Catégoriel c_p^(cat) ∈ R^128 ───────────────────────────
	//
	// TDD §4.1:
	//   c_p^(cat) = (Σ_{h ∈ tags(p)} E_h) / (|tags(p)| + 1)
	//   E_h ∈ R^128 — ligne de la matrice d'embedding SVD
	if opts != nil && len(opts.TagEmbeddings) > 0 {
		computeCatBlock(post.Hashtags, opts.TagEmbeddings, catBlock)
	}
	// Si opts==nil ou TagEmbeddings vide: bloc catégoriel reste à zéro (dégradé gracieux)

	// ── Bloc 2 : Temporel c_p^(temp) ∈ R^24 ────────────────────────────
	//
	// TDD §4.1:
	//   c_{p,k}^(temp) = exp(-(k - h_p)² / (2·σ_h²)) · Z^{-1}
	//   σ_h = 2 h,  Z = Σ_{k=0}^{23} exp(-(k-h_p)²/(2·σ_h²))
	computeTempBlock(post.CreatedAt.Hour(), tempBlock)

	// ── Bloc 3 : Engagement c_p^(eng) ∈ R^8 ────────────────────────────
	//
	// TDD §4.1: "miroir structurel de u^(eng)"
	// À la création: métriques de dwell/scroll à zéro, calcul des métriques disponibles.
	computeEngBlock(post, engBlock)

	// ── Bloc 4 : Social c_p^(soc) ∈ R^64 ───────────────────────────────
	//
	// TDD §4.1:
	//   "embedding du graphe social de l'auteur: identique au mécanisme de u^(soc)
	//   mais centré sur l'auteur du post"
	if opts != nil && len(opts.AuthorSocEmbed) >= variables.VectorDimSoc {
		computeSocBlock(opts.AuthorSocEmbed, socBlock)
	}
	// Si non disponible: bloc social reste à zéro (dégradé gracieux)

	// ── Normalisation L2 finale ─────────────────────────────────────────
	//
	// TDD §2.2:
	//   ĉ_p = c_p / ||c_p||_2
	//   Critique: garantit <ĉ_p, û> ≡ cos(c_p, u) (pas de division au ranking)
	NormalizeL2(vec)

	return vec
}

// computeCatBlock calcule le bloc catégoriel c_p^(cat) ∈ R^128.
//
// TDD §4.1:
//
//	c_p^(cat) = (Σ_{h ∈ tags(p)} E_h) / (|tags(p)| + 1)
//
// Le +1 au dénominateur est intentionnel (TDD): lisse le vecteur même pour les
// posts avec un seul hashtag et évite la division par zéro pour les posts sans tag.
func computeCatBlock(hashtags []string, embeddings map[string][]float32, block []float32) {
	for _, tag := range hashtags {
		normalized := NormalizeHashtag(tag)
		emb, ok := embeddings[normalized]
		if !ok || len(emb) < variables.VectorDimCat {
			continue
		}
		// Σ_{h ∈ tags(p)} E_h — accumulation des embeddings
		for k := 0; k < variables.VectorDimCat; k++ {
			block[k] += emb[k]
		}
	}
	// Division par (|tags(p)| + 1) — TDD §4.1
	divisor := float32(len(hashtags) + 1)
	if divisor > 0 {
		invDiv := float32(1.0) / divisor
		for k := range block {
			block[k] *= invDiv
		}
	}
}

// computeTempBlock calcule le bloc temporel c_p^(temp) ∈ R^24.
//
// TDD §4.1:
//
//	c_{p,k}^(temp) = exp(-(k - h_p)² / (2·σ_h²)) · Z^{-1}
//	σ_h = 2 h,  Z = Σ_{k=0}^{23} exp(-(k-h_p)²/(2·σ_h²))
//
// Encode l'heure de publication comme une distribution de probabilité gaussienne
// sur le cycle journalier (24 heures).
func computeTempBlock(hour int, block []float32) {
	// 2·σ_h² (dénominateur de l'exponentielle)
	const twoSigmaSquared = 2.0 * variables.TDDSigmaHours * variables.TDDSigmaHours // = 8.0

	var sumZ float64
	for k := 0; k < 24; k++ {
		// exp(-(k - h_p)² / (2·σ_h²))
		diff := float64(k - hour)
		val := math.Exp(-(diff * diff) / twoSigmaSquared)
		block[k] = float32(val)
		sumZ += val
	}

	// Normalisation: · Z^{-1} pour obtenir une distribution de probabilité
	if sumZ > 1e-12 {
		invZ := float32(1.0 / sumZ)
		for k := 0; k < 24; k++ {
			block[k] *= invZ
		}
	}
}

// computeEngBlock calcule le bloc engagement c_p^(eng) ∈ R^8.
//
// TDD §4.1: "miroir structurel de u^(eng)"
//
//	u^(eng) = [τ̄_dwell, σ_τ, r_like, r_comment, r_scroll_deep, r_profile_visit, n̄_session, d̄_session]
//
// Les dimensions 0,1,4,5 nécessitent des données de session (zéro à la création,
// mises à jour de manière asynchrone). Les dimensions 2,3,6,7 sont calculées
// depuis les métriques disponibles dans domain.PostRequest.
func computeEngBlock(post domain.PostRequest, block []float32) {
	// sigmoid(x) : borne les valeurs dans (0,1) — normalisation TDD §2.2
	sigmoid := func(x float64) float32 {
		return float32(1.0 / (1.0 + math.Exp(-x)))
	}

	views := math.Max(1.0, float64(post.ViewCount))
	likes := math.Max(0.0, float64(post.LikeCount))
	comments := math.Max(0.0, float64(post.CommentCount))

	// [0] τ̄_dwell — temps de dwell moyen (zéro à la création, mise à jour async)
	block[0] = 0.0
	// [1] σ_τ — écart-type dwell (zéro à la création)
	block[1] = 0.0
	// [2] r_like = likes/views — taux de like sur les posts vus
	// ×10 calibre la sigmoid pour que 0.1 (10% like rate) ≈ sigmoid(1.0) ≈ 0.73
	block[2] = sigmoid(likes / views * 10.0)
	// [3] r_comment = comments/max(likes,1) — ratio commentaires/likes
	block[3] = sigmoid(comments / math.Max(1.0, likes) * 5.0)
	// [4] r_scroll_deep — proportion défilement profond (zéro à la création)
	block[4] = 0.0
	// [5] r_profile_visit — visites de profil post-vue (zéro à la création)
	block[5] = 0.0
	// [6] n̄_session proxy — nombre de médias normalisé (max 5 → 1.0)
	mediaCount := float64(len(post.MediaIDs))
	block[6] = float32(math.Min(1.0, mediaCount/5.0))
	// [7] d̄_session proxy — présence binaire de média (richesse du contenu)
	if post.HasMedia {
		block[7] = 0.5
	} else {
		block[7] = 0.0
	}
}

// computeSocBlock copie l'embedding social de l'auteur dans le bloc social du vecteur.
//
// TDD §4.1:
//
//	"identique au mécanisme de u^(soc) mais centré sur l'auteur du post"
func computeSocBlock(authorSocEmbed []float32, block []float32) {
	n := variables.VectorDimSoc
	if len(authorSocEmbed) < n {
		n = len(authorSocEmbed)
	}
	copy(block[:n], authorSocEmbed[:n])
}

// NormalizeL2 normalise le vecteur v à la norme unitaire (in-place).
//
// TDD §2.2:
//
//	û = u / ||u||_2
//
// Cette normalisation est critique: elle garantit que <û, ĉ_p> ≡ cos(u, c_p),
// évitant une division coûteuse lors de chaque calcul de ranking côté serveur.
//
// Si ||v||_2 < ε (vecteur quasi-nul), la fonction retourne sans modification
// (évite NaN).
func NormalizeL2(v []float32) {
	// ||v||_2² = Σ v_k²
	var norm2 float64
	for _, x := range v {
		norm2 += float64(x) * float64(x)
	}
	if norm2 < 1e-12 {
		return // Vecteur quasi-nul: normalisation impossible
	}
	// v_k ← v_k / ||v||_2  ∀k
	invNorm := float32(1.0 / math.Sqrt(norm2))
	for i := range v {
		v[i] *= invNorm
	}
}
