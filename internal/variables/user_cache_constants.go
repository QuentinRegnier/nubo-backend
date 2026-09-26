package variables

// ############################################################################
// # PARAMÈTRES ET CONSTANTES DU DOMAINE UTILISATEUR (CACHE)
// ############################################################################

const (
	// Marqueur utilisé dans le ZSET (UserTimeline) pour indiquer de manière
	// certaine qu'un profil n'a aucune publication et empêcher un "Cache Miss" BDD
	UserTimelineEmptyMarker = "-1"
)
