package algorithm_service

import "math"

// DopamineWave modélise mathématiquement le gradient de dopamine (La Vague).
// Retourne l'affinité/qualité requise à l'index x (comprise entre 0.01 et 1.0).
// Plus le retour est proche de 1, plus le post doit être un "Banger".
// Plus le retour est faible, plus le système favorise l'exploration et la sérendipité.
func DopamineWave(x float64) float64 {
	if x < 0 {
		return 0.0
	}

	const (
		S_plat    = 0.35 // Seuil de stabilisation de la pertinence
		lambda    = 0.05 // Vitesse de descente de l'enveloppe
		A_debut   = 0.45 // Amplitude maximale initiale du chaos
		A_plat    = 0.20 // Amplitude résiduelle du chaos
		nu        = 0.05 // Vitesse d'atténuation de la nervosité
		P_jackpot = 0.50 // Puissance brute du renforcement intermittent
		K         = 28.0 // Fréquence de base moyenne d'apparition des jackpots
		gamma     = 2.5  // Intensité de la distorsion temporelle
		delta     = 12.0 // Vitesse de variation de la distorsion de phase
		p         = 40.0 // Finesse de l'aiguille du pic de dopamine (pair)
		epsilon   = 0.01 // Plancher de sécurité infrastructurel
	)

	// 1. Enveloppe de base E(x)
	E_x := S_plat + (1.0-S_plat)*math.Exp(-lambda*x)

	// 2. Onde complexe Psi(x)
	Psi_x := (math.Sin(math.Sqrt2*x) + math.Sin(math.Pi*x) + math.Sin(math.E*x)) / 3.0

	// 3. Chaos modulé Omega(x)
	Omega_x := (A_plat + (A_debut-A_plat)*math.Exp(-nu*x)) * Psi_x

	// 4. Distorsion de phase Phi(x)
	Phi_x := gamma * math.Sin((x*math.Sqrt2)/delta)

	// 5. Pics de Jackpot J(x)
	cosVal := math.Cos((x*math.Pi)/K + Phi_x)
	var J_x float64
	if cosVal > 0 {
		J_x = P_jackpot * math.Pow(cosVal, p)
	}

	// 6. Assemblage final
	f_x := E_x + Omega_x + J_x

	// Clamping final
	if f_x < epsilon {
		return epsilon
	}
	if f_x > 1.0 {
		return 1.0
	}
	return f_x
}
