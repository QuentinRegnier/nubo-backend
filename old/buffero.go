package old

/*
// ─────────────────────────────────────────────────────────────────────────────
// GESTIONNAIRE DE BUFFER (L'Usine à Feeds - Phase 4)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// Clés Redis pour la pagination du feed_service
	RedisKeyFeedBufferPage = "feed_service:buffer:%d:page:%d"
	RedisKeyFeedCursor     = "feed_service:cursor:%d"

	// Durée de vie du buffer précalculé. En LFU, si le serveur sature,
	// Redis pourra évincer ces clés car elles ont un TTL explicite.
	BufferTTL = 24 * time.Hour
)

// SaveBuffer pages découpe la sélection finale de la "Caissière" en pages distinctes
// et les sauvegarde atomiquement dans Redis. Il réinitialise également le curseur.
func SaveBuffer(ctx context.Context, userID int64, feedIDs []int64, pageSize int) error {
	if pageSize <= 0 {
		pageSize = 50 // Taille par défaut d'une page (variables.TDDFeedSize)
	}

	// Utilisation d'un Pipeline pour éviter les allers-retours réseau.
	// Toutes les commandes partent en un seul paquet TCP.
	pipe := redisgo.Rdb.Pipeline()

	// 1. Initialisation du curseur à la page 1
	cursorKey := fmt.Sprintf(RedisKeyFeedCursor, userID)
	pipe.Set(ctx, cursorKey, 1, BufferTTL)

	// 2. Nettoyage préventif des potentielles anciennes pages "fantômes"
	// Si l'ancien buffer avait 10 pages et le nouveau 3, on ne veut pas garder les pages 4 à 10.
	// On envoie une vague de DEL (très peu coûteux en Pipeline).
	for i := 1; i <= 20; i++ {
		pipe.Del(ctx, fmt.Sprintf(RedisKeyFeedBufferPage, userID, i))
	}

	// 3. Découpage et sauvegarde des nouvelles pages
	totalPages := (len(feedIDs) + pageSize - 1) / pageSize
	for page := 1; page <= totalPages; page++ {
		start := (page - 1) * pageSize
		end := start + pageSize
		if end > len(feedIDs) {
			end = len(feedIDs)
		}

		pageIDs := feedIDs[start:end]
		pageKey := fmt.Sprintf(RedisKeyFeedBufferPage, userID, page)

		// Note: On utilise json.Marshal ici car c'est un simple []int64.
		// Tu pourras basculer sur msgpack.Marshal() en 1 clic pour gratter
		// quelques octets de RAM si l'infrastructure le requiert.
		data, err := json.Marshal(pageIDs)
		if err != nil {
			return fmt.Errorf("erreur sérialisation page %d: %w", page, err)
		}

		pipe.Set(ctx, pageKey, data, BufferTTL)
	}

	// 4. Exécution atomique
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("erreur pipeline SaveBuffer: %w", err)
	}

	return nil
}

// GetCurrentCursor lit la position actuelle du curseur de l'utilisateur.
// Utile pour l'étape 4.2 (Pull-to-refresh vs Scroll continu).
func GetCurrentCursor(ctx context.Context, userID int64) int {
	cursorKey := fmt.Sprintf(RedisKeyFeedCursor, userID)
	val, err := redisgo.Rdb.Get(ctx, cursorKey).Result()
	if err != nil {
		return 1 // Par défaut, si le curseur a expiré ou n'existe pas, on pointe sur la page 1
	}

	page, _ := strconv.Atoi(val)
	if page < 1 {
		return 1
	}
	return page
}

// IncrementCursor déplace le curseur à la page suivante (appelé quand l'utilisateur a fini de consommer la page).
func IncrementCursor(ctx context.Context, userID int64) {
	cursorKey := fmt.Sprintf(RedisKeyFeedCursor, userID)
	redisgo.Rdb.Incr(ctx, cursorKey)
}

// GetBufferPage récupère une page spécifique d'IDs de posts stockée dans le buffer Redis.
func GetBufferPage(ctx context.Context, userID int64, page int) ([]int64, error) {
	pageKey := fmt.Sprintf(RedisKeyFeedBufferPage, userID, page)

	data, err := redisgo.Rdb.Get(ctx, pageKey).Bytes()
	if err != nil {
		return nil, fmt.Errorf("page %d indisponible ou expirée : %w", page, err)
	}

	var postIDs []int64
	if err := json.Unmarshal(data, &postIDs); err != nil {
		return nil, fmt.Errorf("erreur de désérialisation de la page %d : %w", page, err)
	}

	return postIDs, nil
}

// ClearBuffer nettoie de manière atomique toutes les pages potentielles et le curseur d'un utilisateur.
func ClearBuffer(ctx context.Context, userID int64) error {
	pipe := redisgo.Rdb.Pipeline()

	cursorKey := fmt.Sprintf(RedisKeyFeedCursor, userID)
	pipe.Del(ctx, cursorKey)

	// Nettoyage des 20 pages de sécurité par précaution
	for i := 1; i <= 20; i++ {
		pipe.Del(ctx, fmt.Sprintf(RedisKeyFeedBufferPage, userID, i))
	}

	_, err := pipe.Exec(ctx)
	return err
}
*/
