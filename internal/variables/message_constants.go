package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU DOMAINE MESSAGES ET NOTIFICATIONS
// ############################################################################

const (
	// Types de messages
	MessageTypeText   = 0
	MessageTypeVoice  = 1
	MessageTypeMedia  = 2
	MessageTypeGIF    = 3
	MessageTypeVideo  = 4
	MessageTypePost   = 5
	MessageTypeInvite = 6
	MessageTypeLink   = 7
	MessageTypeSystem = 8
	MessageTypeSurvey = 9

	// États de restriction des notifications (IsMuted)
	MuteStatusNone         = 0 // Notifications normales
	MuteStatusMentionsOnly = 1 // Sourdine partielle (Seules les mentions notifient)
	MuteStatusFull         = 2 // Sourdine totale (Aucune notification)
)
