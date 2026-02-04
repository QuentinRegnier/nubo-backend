package variables

const (
	ToleranceTimeSeconds         = 300     // 5 minutes
	JWTExpirationSeconds         = 900     // 15 minutes
	MasterTokenExpirationSeconds = 2592000 // 1 mois en secondes
)

// Configuration des bits pour l'algorithme Snowflake
const (
	Epoch    = int64(1704067200000) // Date de départ : 1er Janvier 2024 (Custom Epoch)
	NodeBits = uint(10)             // 10 bits pour l'ID du noeud (1024 noeuds max)
	StepBits = uint(12)             // 12 bits pour la séquence (4096 IDs par ms)

	NodeMax   = int64(-1 ^ (-1 << NodeBits)) // Max Node ID (1023)
	StepMax   = int64(-1 ^ (-1 << StepBits)) // Max Sequence (4095)
	TimeShift = NodeBits + StepBits          // Décalage pour le timestamp (22)
	NodeShift = StepBits                     // Décalage pour le noeud (12)
)

const MaxTags = 10
