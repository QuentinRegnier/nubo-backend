package variables

// Configuration des bits pour l'algorithme Snowflake
const (
	Epoch    = int64(1704067200000) // Date de départ : 1er Janvier 2024 (Custom Epoch)
	nodeBits = uint(10)             // 10 bits pour l'ID du noeud (1024 noeuds max)
	stepBits = uint(12)             // 12 bits pour la séquence (4096 IDs par ms)

	NodeMax   = int64(-1 ^ (-1 << nodeBits)) // Max Node ID (1023)
	StepMax   = int64(-1 ^ (-1 << stepBits)) // Max Sequence (4095)
	TimeShift = nodeBits + stepBits          // Décalage pour le timestamp (22)
	NodeShift = stepBits                     // Décalage pour le noeud (12)
)
