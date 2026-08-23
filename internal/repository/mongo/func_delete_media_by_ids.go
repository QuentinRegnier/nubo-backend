package mongo

// MongoDeleteMediaByIDs effectue un Hard Delete sur le stockage à froid (L2)
func MongoDeleteMediaByIDs(ids []int64) error {
	if len(ids) == 0 || Media == nil {
		return nil
	}

	// Utilisation de la méthode Delete abstraite du wrapper (DDD)
	filter := map[string]any{"id": map[string]any{"$in": ids}}
	return Media.Delete(filter)
}
