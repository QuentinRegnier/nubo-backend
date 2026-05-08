package variables

import "time"

// Configuration temporelle OBJECT Cache
const (
	StandardTTL = 7 * 24 * time.Hour // TTL STANDARD : 7 Jours (Pragmatique, évite la saturation).
)
