package variables

const (
	GraphDecayLambda   = 0.05  // λ : Taux de perte de pertinence (Ex: 5% d'oubli par jour)
	GraphLearningAlpha = 0.20  // α : Force d'apprentissage d'une nouvelle co-occurrence (20%)
	GraphSurvivalEps   = 0.001 // ε : Seuil d'oubli absolu (En dessous, le segment est détruit)
)
