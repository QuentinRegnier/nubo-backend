package mongo

// MongoGetPostLikes interroge le stockage L2 pour la liste des likes.
func MongoGetPostLikes(postID int64, limit int, offset int) ([]int64, error) {
	// ⚠️ CORRECTION POLYMORPHE : On cible explicitement les Posts (0) et l'ID
	filter := map[string]any{
		"target_type": 0,
		"target_id":   postID,
	}

	// On trie par date de création décroissante (les plus récents en premier)
	sort := map[string]any{"created_at": -1}

	docs, err := Likes.GetPaginated(filter, sort, int64(offset), int64(limit))
	if err != nil {
		return nil, err
	}

	var userIDs []int64
	for _, doc := range docs {
		if uidFloat, ok := doc["user_id"].(float64); ok {
			userIDs = append(userIDs, int64(uidFloat))
		} else if uidInt64, ok := doc["user_id"].(int64); ok {
			userIDs = append(userIDs, uidInt64)
		} else if uidInt32, ok := doc["user_id"].(int32); ok {
			userIDs = append(userIDs, int64(uidInt32))
		}
	}

	return userIDs, nil
}
