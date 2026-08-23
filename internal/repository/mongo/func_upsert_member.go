package mongo

import (
	"fmt"

	"github.com/QuentinRegnier/nubo-backend/internal/domain/models/conversation_models"
	"github.com/QuentinRegnier/nubo-backend/internal/pkg"
)

// MongoUpsertMember insère ou met à jour un membre complet dans le Cold Storage L2.
func MongoUpsertMember(mem conversation_models.MemberPayload) error {
	doc, err := pkg.ToMap(mem)
	if err != nil || doc == nil {
		return fmt.Errorf("erreur conversion member_payload pour Mongo")
	}
	// On utilise Set car MemberPayload possède un ID Snowflake unique, comme les posts et users
	return Members.Set(doc)
}
