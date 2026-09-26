package algorithm_service

import "math"

// ############################################################################
// # LA VAGUE DE DOPAMINE (Gradient Modulé Mathématiquement)
// ############################################################################

// DopamineWave modélise le gradient de dopamine.
// Retourne l'affinité/qualité requise à l'index x (comprise entre 0.01 et 1.0).
// - Proche de 1 : Le système exige un "Banger" très pertinent.
// - Proche de 0 : Le système favorise l'exploration, l'aléatoire et la sérendipité.
func DopamineWave(indexScroll float64) float64 {
	if indexScroll < 0 {
		return 0.0
	}

	// TABLE DE MIXAGE DES FRÉQUENCES
	const (
		BasePlateau          = 0.35 // S_plat : Seuil de stabilisation de la pertinence (Ne descend jamais en dessous)
		DecaySpeed           = 0.05 // lambda : Vitesse de descente de l'enveloppe initiale
		InitialAmplitude     = 0.45 // A_debut : Amplitude maximale du chaos au début du scroll
		ResidualAmplitude    = 0.20 // A_plat : Amplitude résiduelle du chaos au fond du scroll
		NervousnessDecay     = 0.05 // nu : Vitesse d'atténuation de la "nervosité" de la courbe
		JackpotPower         = 0.50 // P_jackpot : Puissance brute du renforcement intermittent (Les Pics de Qualité)
		JackpotFrequency     = 28.0 // K : Fréquence de base moyenne d'apparition des jackpots
		TimeDistortionGamma  = 2.5  // gamma : Intensité de la distorsion temporelle (Le scroll est imprévisible)
		PhaseDistortionDelta = 12.0 // delta : Vitesse de variation de la distorsion de phase
		SpikeSharpness       = 40.0 // p : Finesse de l'aiguille du pic de dopamine (Doit être un nombre pair)
		FloorSafety          = 0.01 // epsilon : Plancher de sécurité infrastructurel
	)

	// 1. Enveloppe de Base E(x) (L'attraction vers le plateau de 35% de pertinence)
	baseEnvelope := BasePlateau + (1.0-BasePlateau)*math.Exp(-DecaySpeed*indexScroll)

	// 2. Onde Complexe Psi(x) (Combinaison de sinus sur des nombres irrationnels pour éviter les patterns)
	complexWave := (math.Sin(math.Sqrt2*indexScroll) + math.Sin(math.Pi*indexScroll) + math.Sin(math.E*indexScroll)) / 3.0

	// 3. Chaos Modulé Omega(x) (L'amplitude du chaos diminue doucement avec le scroll)
	modulatedChaos := (ResidualAmplitude + (InitialAmplitude-ResidualAmplitude)*math.Exp(-NervousnessDecay*indexScroll)) * complexWave

	// 4. Distorsion de Phase Phi(x) (La distance entre les jackpots n'est jamais la même)
	phaseDistortion := TimeDistortionGamma * math.Sin((indexScroll*math.Sqrt2)/PhaseDistortionDelta)

	// 5. Pics de Jackpot J(x) (L'injection massive et brutale d'un contenu excellent)
	cosValue := math.Cos((indexScroll*math.Pi)/JackpotFrequency + phaseDistortion)
	var jackpotSpike = 0.0
	if cosValue > 0 {
		jackpotSpike = JackpotPower * math.Pow(cosValue, SpikeSharpness)
	}

	// 6. Assemblage Final
	finalWaveValue := baseEnvelope + modulatedChaos + jackpotSpike

	// Clamping final (Sécurité Mathématique)
	if finalWaveValue < FloorSafety {
		return FloorSafety
	}
	if finalWaveValue > 1.0 {
		return 1.0
	}

	return finalWaveValue
}
