package variables

// ─────────────────────────────────────────────────────────────────────────────
// TYPES DE CIBLES (Target Types)
// ─────────────────────────────────────────────────────────────────────────────
const (
	ReportTargetPost         = 0
	ReportTargetComment      = 1
	ReportTargetConversation = 2
	ReportTargetMessage      = 3
	ReportTargetUser         = 5 // Comme tu l'as défini
)

// ─────────────────────────────────────────────────────────────────────────────
// CATÉGORIES DE SIGNALEMENT (Précises, sans "super-catégorie")
// ─────────────────────────────────────────────────────────────────────────────
const (
	ReportCatSpam             = 1
	ReportCatHarassmentMoral  = 2  // Harcèlement moral / Cyberintimidation
	ReportCatHarassmentSexual = 3  // Harcèlement sexuel
	ReportCatHateSpeech       = 4  // Incitation à la haine / Violence
	ReportCatIdentityTheft    = 5  // Usurpation d'identité
	ReportCatUnderage         = 6  // Présence d'un mineur (Critique sur une appli Naturiste)
	ReportCatNonConsensual    = 7  // Partage de contenu non consenti
	ReportCatIllegalContent   = 8  // Contenu illégal (Drogue, Armes, etc.)
	ReportCatSelfHarm         = 9  // Automutilation / Suicide
	ReportCatOther            = 99 // Autre (se référer au champ "reason")
)

// ─────────────────────────────────────────────────────────────────────────────
// ETAT DU SIGNALEMENT (State)
// ─────────────────────────────────────────────────────────────────────────────
const (
	ReportStatePending    = 0  // En attente de traitement
	ReportStateInProgress = 1  // En cours de traitement par un modérateur
	ReportStateEscalated  = 2  // Escaladé à un supérieur
	ReportStateClosed     = -1 // Fermé / Traité
)
