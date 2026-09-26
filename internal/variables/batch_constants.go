package variables

// ============================================================================
// CONSTANTES SPÉCIFIQUES AU BATCH MONGODB
// ============================================================================
const (
	// MongoBulkWriteOrdered définit si l'échec d'une opération doit bloquer les suivantes.
	// False permet de traiter le maximum d'événements, même si un élément du lot échoue.
	MongoBulkWriteOrdered = false
)
