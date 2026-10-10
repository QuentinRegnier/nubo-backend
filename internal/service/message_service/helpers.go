package message_service

import (
	"context"

	"github.com/QuentinRegnier/numan-backend/internal/pkg"

	"github.com/QuentinRegnier/numan-backend/internal/repository/redis"
)

// ############################################################################
// # UTILITAIRES : RÉCUPÉRATION ET SYNCHRONISATION
// ############################################################################

// getParticipantIDsForLedgerSync récupère les identifiants actifs de la conversation
// en O(1) depuis la RAM pour déclencher le Sync Ledger des clients connectés.
func getParticipantIDsForLedgerSync(ctx context.Context, conversationID int64) []int64 {
	participantsStringList, err := redis.ConvParticipants.SMembers(ctx, conversationID)
	if err != nil || len(participantsStringList) == 0 {
		return nil
	}

	participantIDs := pkg.ParseInt64List(participantsStringList)

	return participantIDs
}
