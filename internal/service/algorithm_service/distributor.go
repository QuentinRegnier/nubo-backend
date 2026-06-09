package algorithm_service

import (
	"context"
	"math/rand"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/service"
	"github.com/QuentinRegnier/nubo-backend/internal/variables"
)

// ✅ AJOUT DE L'INSTANCE GLOBALE POUR LE CLEAN ARCHITECTURE
var GlobalDistributor *FeedDistributor

// FeedDistributor orchestre la distribution et le cycle de vie du flux d'actualité.
type FeedDistributor struct {
	clerk *ProtoFeedBuilder
}

// NewFeedDistributor initialise le distributeur de flux.
func NewFeedDistributor(clerk *ProtoFeedBuilder) *FeedDistributor {
	return &FeedDistributor{
		clerk: clerk,
	}
}

// RefreshOptions encapsule les données de contexte envoyées par le client mobile/web.
type RefreshOptions struct {
	UserID        int64
	LastSeenIndex int
	FetchCount    int // ✅ Permet au Service de demander une quantité précise pour combler les trous
	Quotas        Quotas
	PersonalOpts  PersonalizedFeedOptions
}

// HandlePullToRefresh applique les règles psychologiques et techniques du Swipe Down.
func (d *FeedDistributor) HandlePullToRefresh(ctx context.Context, opts RefreshOptions) ([]int64, error) {
	// Seuil critique métier issu des spécifications (§4.2)
	const DislikeThreshold = 10

	// ─────────────────────────────────────────────────────────────────────────────
	// CHARGEMENT OU INITIALISATION DE L'ÉTAT DU FEED
	// ─────────────────────────────────────────────────────────────────────────────
	state, err := GetUserFeedState(ctx, opts.UserID)
	if err != nil {
		state = FeedState{
			ActiveFeed: "A",
			Feeds: map[string]FeedData{
				"A": {Seed: rand.Int63()},
				"B": {Seed: rand.Int63()},
				"C": {Seed: rand.Int63()},
			},
		}
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// CAS 1 : REJET DU FEED (last_seen_index < 10) OU GÉNÉRATION FORCÉE
	// ─────────────────────────────────────────────────────────────────────────────
	if opts.LastSeenIndex < DislikeThreshold {
		// ✅ ARCHIVAGE INTERNE DANS LE CUCKOO FILTER
		// Avant de détruire le flux actuel, on inscrit tous les IDs distribués (jusqu'au curseur)
		// dans le filtre RedisBloom de l'utilisateur pour une amnésie à long terme.
		if len(state.Feeds[state.ActiveFeed].PostIDs) > 0 {
			endIdx := opts.LastSeenIndex
			if endIdx > len(state.Feeds[state.ActiveFeed].PostIDs) {
				endIdx = len(state.Feeds[state.ActiveFeed].PostIDs)
			}
			distributedIDs := state.Feeds[state.ActiveFeed].PostIDs[0:endIdx]

			for _, postID := range distributedIDs {
				service.MarkAsSeen(ctx, opts.UserID, postID) // ✅ Retrait du "_ =" car la fonction ne retourne rien
			}
		}

		// ✅ UTILISATION DE LA CONSTANTE GLOBALE
		if time.Since(state.GeneratedAt) >= variables.FeedReloadDelay {
			// ✅ CAS 1 (/force) : Temps écoulé -> Reset et nouvelle génération
			state.GeneratedAt = time.Now()
			state.ActiveFeed = "A"
			state.Feeds["A"] = FeedData{Seed: rand.Int63(), PostIDs: nil, Fused: false}
			state.Feeds["B"] = FeedData{Seed: rand.Int63(), PostIDs: nil, Fused: false}
			state.Feeds["C"] = FeedData{Seed: rand.Int63(), PostIDs: nil, Fused: false}

			seeds := [3]int64{state.Feeds["A"].Seed, state.Feeds["B"].Seed, state.Feeds["C"].Seed}
			baskets, _ := d.clerk.CollectCandidates(ctx, opts.UserID, seeds, opts.Quotas)

			var candidateIDs []int64
			for _, item := range baskets.A.Items {
				candidateIDs = append(candidateIDs, item.PostID)
			}
			opts.PersonalOpts.CandidateIDs = candidateIDs
			opts.PersonalOpts.Seed = state.Feeds["A"].Seed
			opts.PersonalOpts.StartIndex = 0

			freshFeed, _ := BuildPersonalizedFeed(ctx, opts.PersonalOpts)
			feedData := state.Feeds["A"]
			feedData.PostIDs = freshFeed
			state.Feeds["A"] = feedData
		} else {
			// ✅ CAS 2 (/force) : Moins de 30 min -> Rotation Circulaire
			switch state.ActiveFeed {
			case "A":
				state.ActiveFeed = "B"
			case "B":
				state.ActiveFeed = "C"
			case "C":
				state.ActiveFeed = "A"
			}
		}

		_ = SaveUserFeedState(ctx, opts.UserID, state)
		opts.LastSeenIndex = 0 // On force à 0, et on laisse couler vers le bloc de Consommation
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// CAS 2 : CONSOMMATION NORMALE (last_seen_index >= 10)
	// ─────────────────────────────────────────────────────────────────────────────
	activeData := state.Feeds[state.ActiveFeed]
	totalPosts := len(activeData.PostIDs)
	remaining := totalPosts - opts.LastSeenIndex

	// ── Cas 1 (Pagination simple) : Assez de posts en stock ───────────────────
	if remaining >= opts.FetchCount {
		endIndex := opts.LastSeenIndex + opts.FetchCount
		return activeData.PostIDs[opts.LastSeenIndex:endIndex], nil
	}

	// ── Cas 2 & 3 : Besoin d'étendre la liste ─────────────────────────────────
	var candidateIDs []int64

	// ✅ FILTRE DE SÉCURITÉ : On mémorise les posts déjà présents dans le flux actif pour éviter tout doublon
	seenInActiveFeed := make(map[int64]struct{}, len(activeData.PostIDs))
	for _, id := range activeData.PostIDs {
		seenInActiveFeed[id] = struct{}{}
	}

	if !activeData.Fused {
		// Cas 2 (Fusion) : On recycle le travail du Magasinier via les paniers inactifs
		for feedName, feedData := range state.Feeds {
			if feedName != state.ActiveFeed {
				for _, id := range feedData.PostIDs {
					// On n'ajoute que si le post n'est pas déjà dans le flux Actif
					if _, exists := seenInActiveFeed[id]; !exists {
						candidateIDs = append(candidateIDs, id)
						seenInActiveFeed[id] = struct{}{} // Empêche aussi les doublons entre B et C
					}
				}
			}
		}
		activeData.Fused = true
	} else {
		// Cas 3 (Re-sélection / Extension) : Les paniers sont déjà absorbés
		singleBasket, err := d.clerk.CollectSingleBasket(ctx, opts.UserID, activeData.Seed, opts.Quotas)
		if err != nil {
			return nil, err
		}
		for _, item := range singleBasket.Items {
			// Même sécurité pour le nouveau panier généré
			if _, exists := seenInActiveFeed[item.PostID]; !exists {
				candidateIDs = append(candidateIDs, item.PostID)
				seenInActiveFeed[item.PostID] = struct{}{}
			}
		}
	}

	// ✅ LIAISON MAGASINIER -> CAISSIÈRE AVEC DÉCALAGE (StartIndex)
	opts.PersonalOpts.CandidateIDs = candidateIDs
	opts.PersonalOpts.Seed = activeData.Seed
	opts.PersonalOpts.StartIndex = totalPosts // On garantit la continuité de la Vague de Dopamine

	// Passage en Caisse pour trier la suite de la liste et injecter la sérendipité
	freshFeed, err := BuildPersonalizedFeed(ctx, opts.PersonalOpts)
	if err != nil {
		return nil, err
	}

	// On greffe la suite générée au flux actif existant
	activeData.PostIDs = append(activeData.PostIDs, freshFeed...)
	state.Feeds[state.ActiveFeed] = activeData
	_ = SaveUserFeedState(ctx, opts.UserID, state)

	// Sécurité de slice final
	newTotal := len(activeData.PostIDs)

	// Sécurité absolue : la base de données globale de l'application est litéralement vide (ex: jour de lancement)
	if newTotal == 0 {
		return []int64{}, nil
	}

	// ✅ L'ÉLÉPHANT EST LÀ : L'utilisateur a vu 100% du Most Cache.
	// Stratégie de l'Amnésie (Instagram) : On le bascule en douceur sur un autre flux.
	// La nouvelle Seed va re-scorer les posts et inverser les pics de Dopamine.
	if opts.LastSeenIndex >= newTotal {
		switch state.ActiveFeed {
		case "A":
			state.ActiveFeed = "B"
		case "B":
			state.ActiveFeed = "C"
		case "C":
			state.ActiveFeed = "A"
		}
		_ = SaveUserFeedState(ctx, opts.UserID, state)

		// On pioche instantanément le haut du nouveau panier
		rotatedFeed := state.Feeds[state.ActiveFeed].PostIDs
		end := opts.FetchCount
		if end > len(rotatedFeed) {
			end = len(rotatedFeed)
		}
		return rotatedFeed[0:end], nil
	}

	endIndex := opts.LastSeenIndex + opts.FetchCount
	if endIndex > newTotal {
		endIndex = newTotal // Prévention Out-Of-Bounds
	}

	return activeData.PostIDs[opts.LastSeenIndex:endIndex], nil
}
