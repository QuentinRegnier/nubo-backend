package auth_service

import (
	"context"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/auth_models"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/mongo"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/postgres"
	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/cache_service/object_cache_service"
	"github.com/QuentinRegnier/nubo-backend/internal/service/media_service"
)

// SyncIdentity synchronise uniquement l'identité (Profil et Settings) pour le Cold Start de l'App.
func SyncIdentity(ctx context.Context, input auth_models.SyncIdentityInput) (auth_models.SyncIdentityOutput, error) {
	output := auth_models.SyncIdentityOutput{
		ProfileUpdated:  false,
		SettingsUpdated: false,
	}

	// =====================================================================
	// 1. DELTA SYNC : PROFIL UTILISATEUR (L2 -> L3)
	// Pas de L1 ici car le L1 ne stocke pas les données privées (Email, Phone)
	// =====================================================================
	user, errUser := mongo.MongoLoadUser(input.UserID, "", "", "")
	if errUser != nil || user.ID == 0 {
		user, _ = postgres.FuncLoadUser(input.UserID, "", "", "")

		if user.ID != 0 {
			// ⬆️ PROMOTION L3 -> L2 (Asynchrone via Worker)
			// L'hydratation L1 du SpeedCache se fera automatiquement si l'utilisateur
			// met à jour son profil plus tard.
			go func(u auth_models.UserPayload) {
				bgCtx := context.Background()
				_ = redis.EnqueueDB(bgCtx, u.ID, 0, redis.EntityUser, redis.ActionUpdate, u, redis.TargetMongo)
			}(user)
		}
	}

	if user.ID != 0 {
		if user.UpdatedAt > input.ProfileUpdatedAt {
			output.ProfileUpdated = true

			// ✅ MAPPING SÉCURISÉ : On transfère uniquement les champs autorisés
			output.Profile = auth_models.UserProfileView{
				ID:        user.ID,
				Username:  user.Username,
				Email:     user.Email,
				Phone:     user.Phone,
				FirstName: user.FirstName,
				LastName:  user.LastName,
				Birthdate: user.Birthdate,
				Sex:       user.Sex,
				Bio:       user.Bio,
				Grade:     user.Grade,
				Location:  user.Location,
				School:    user.School,
				Work:      user.Work,
				Badges:    user.Badges,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
				IsOnline:  cache_service.IsUserOnline(ctx, user.ID), // O(1) L1 Call
			}

			// === HYDRATATION DE L'AVATAR (Composition par Valeur) ===
			if user.ProfilePictureID > 0 {
				if view, err := media_service.GenerateMediaViewCascade(ctx, user.ProfilePictureID, user.ID, 0, input.UserID); err == nil {
					output.Avatar = view
				}
			}
		}
	}

	// =====================================================================
	// 2. DELTA SYNC : PARAMÈTRES (Cascade complète existante)
	// =====================================================================
	settings, errSettings := object_cache_service.GetUserSettingsCascade(ctx, input.UserID)
	if errSettings == nil && settings.ID != 0 {
		if settings.UpdatedAt > input.SettingsUpdatedAt {
			output.SettingsUpdated = true
			output.Settings = settings
		}
	}

	return output, nil
}
