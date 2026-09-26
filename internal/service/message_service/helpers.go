package message_service

import (
	"context"
	"strconv"

	"github.com/QuentinRegnier/nubo-backend/internal/repository/redis"
)

// ############################################################################
// # UTILITAIRES : RÉCUPÉRATION ET SYNCHRONISATION
// ############################################################################

// GetParticipantIDsForLedgerSync récupère les identifiants actifs de la conversation
// en O(1) depuis la RAM pour déclencher le Sync Ledger des clients connectés.
func GetParticipantIDsForLedgerSync(ctx context.Context, conversationID int64) []int64 {
	participantsStringList, err := redis.ConvParticipants.SMembers(ctx, conversationID)
	if err != nil || len(participantsStringList) == 0 {
		return nil
	}

	participantIDs := make([]int64, 0, len(participantsStringList))
	for _, participantStr := range participantsStringList {
		if parsedID, errParse := strconv.ParseInt(participantStr, 10, 64); errParse == nil {
			participantIDs = append(participantIDs, parsedID)
		}
	}

	return participantIDs
}
