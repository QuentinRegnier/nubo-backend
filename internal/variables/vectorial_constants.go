package variables

// ================================================================
// DIMENSIONS VECTORIELLES — TDD §2.2 / §6
// u = [u^(cat) | u^(temp) | u^(eng) | u^(soc)] ∈ R^224
// ================================================================
const (
	VectorDimTotal = 224 // N — dimension totale
	VectorDimCat   = 128 // N_cat — catégoriel SVD hashtags    [0:128)
	VectorDimTemp  = 24  // N_temp — temporel heure journalière [128:152)
	VectorDimEng   = 8   // N_eng  — engagement comportemental  [152:160)
	VectorDimSoc   = 64  // N_soc  — graphe social              [160:224)

	// Offsets de début de chaque bloc dans le vecteur complet
	VectorOffCat  = 0   // début bloc catégoriel
	VectorOffTemp = 128 // début bloc temporel
	VectorOffEng  = 152 // début bloc engagement
	VectorOffSoc  = 160 // début bloc social
)
