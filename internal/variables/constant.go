package variables

const (
	ToleranceTimeSeconds         = 300     // 5 minutes
	JWTExpirationSeconds         = 900     // 15 minutes
	MasterTokenExpirationSeconds = 2592000 // 1 mois en secondes
)

// ---------------------------------------------------------
// RECOMMANDATION - MULTIPLICATEURS (BOOSTS)
// ---------------------------------------------------------
const (
	BoostRecent   = 1.8 // Réduit la gravité temporelle pour favoriser la fraîcheur
	BoostLikes    = 1.5 // Augmente le poids des likes de 50%
	BoostComments = 1.5 // Augmente le poids des commentaires de 50%
)
