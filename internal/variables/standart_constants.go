package variables

const (
	UserGradeUser      = 0
	UserGradeCertified = 1
	UserGradeParter    = 2
	UserGradeModerator = 3
	UserGradeAdmin     = 4
)

const (
	FetchBatchLimit       = 50
	MaxTimestamp    int64 = 1<<63 - 1
)

const (
	ToleranceTimeSeconds         = 300     // 5 minutes
	JWTExpirationSeconds         = 900     // 15 minutes
	MasterTokenExpirationSeconds = 2592000 // 1 mois en secondes
)
