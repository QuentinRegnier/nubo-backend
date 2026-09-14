package api

import (
	"net/http"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/auth_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/comment_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/conversation_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/feed_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/like_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/media_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/message_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/notification_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/post_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/relation_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/report_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/saved_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/search_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/security_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/telemetry_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/user_settings_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/websocket"
	"github.com/golang-jwt/jwt/v5"

	"github.com/QuentinRegnier/nubo-backend/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// =========================================================================
	// 0. Middleware GLOBAL (S'applique à TOUTES les routes)
	// =========================================================================

	// 1. GUILLOTINE : Coupe les requêtes obèses avant même de les lire en RAM (Protège la RAM)
	r.Use(middleware.MaxBodySize())

	// 2. ANTI-SPAM : Bloque les attaques DDoS applicatives via Redis (Protège le CPU et BDD)
	r.Use(middleware.RateLimiter())

	// 3. CORS : Active la gestion automatique des OPTIONS et du CORS
	r.Use(middleware.CORSMiddleware())

	// 4. RECOVERY : Récupération automatique des panics (évite que le serveur crash totalement)
	r.Use(gin.Recovery())

	// =========================================================================
	// 1. ROUTES PUBLIQUES (Aucune sécu ou sécu spécifique interne)
	// =========================================================================

	// Authentification (Sécu interne spécifique)
	r.POST("/signup", auth_handlers.SignUpHandler)                         //
	r.POST("/login", auth_handlers.LoginHandler)                           //
	r.POST("/check-username", user_settings_handlers.CheckUsernameHandler) //

	// Renouvellement de Tokens (Ratchet / Master)
	// Ces routes gèrent leur propre sécurité (HMAC spécial, checks BDD...)
	r.POST("/renew-jwt", security_handlers.RenewJWT)
	r.POST("/refresh-master", security_handlers.RefreshMaster)

	// WebSocket
	r.GET("/token", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": 1234,
			"dev": "device-token-sample",
			"exp": time.Now().Add(time.Hour * 24).Unix(), // expire dans 24h
			"iat": time.Now().Unix(),
		})
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			panic("JWT_SECRET manquant dans .env")
		}
		tokenString, _ := token.SignedString([]byte(secret))
		c.JSON(200, gin.H{"token": tokenString})
	})

	// =========================================================================
	// 2. ROUTES SÉCURISÉES (JWT + HMAC + RATCHET)
	// Toutes les routes ci-dessous nécessitent :
	// - Un JWT valide (Identity)
	// - Une signature HMAC valide (Integrity & Anti-Replay)
	// =========================================================================

	// On crée un groupe "plat" qui applique les deux middlewares d'un coup
	secured := r.Group("/")
	secured.Use(middleware.JWTMiddleware())  // 1. Qui est-ce ? (Populate context with UserID & FirebaseInstallationIDs)
	secured.Use(middleware.HMACMiddleware()) // 2. Est-ce authentique ? (Check Signature with Redis Secret)

	// --- Websocket ---
	secured.GET("/ws", websocket.ServeWS) //

	// --- User ---
	secured.POST("/logout", auth_handlers.LogoutHandler)                  //
	secured.GET("/session/get", auth_handlers.GetSessionsHandler)         //
	secured.DELETE("/session/delete", auth_handlers.DeleteSessionHandler) //

	// --- Posts ---
	secured.POST("/feed/get", feed_handlers.GetFeedHandler) //
	secured.POST("/posts/get", post_handlers.GetPostHandler)
	secured.POST("/posts/user/get", post_handlers.GetUserPostsHandler) // Profil d'un utilisateur (non)
	secured.POST("/posts/set", post_handlers.CreatePostHandler)        //
	secured.PUT("/posts/update", post_handlers.UpdatePostHandler)      //
	secured.DELETE("/posts/delete", post_handlers.DeletePostHandler)   //

	// --- Notifications ---
	secured.POST("/notifications/get", notification_handlers.GetNotificationsHandler) //
	secured.POST("/notifications/read", notification_handlers.ReadNotificationsHandler)

	// --- Sync ---
	secured.PATCH("/sync/telemetry", telemetry_handlers.SyncTelemetryHandler) //
	secured.POST("/sync/inbox", conversation_handlers.SyncInboxHandler)       //
	secured.POST("/sync/identity", auth_handlers.SyncIdentityHandler)         //
	secured.POST("/sync/activity", notification_handlers.SyncActivityHandler) //

	// --- Actions Sociales ---
	secured.POST("/like/post/set", like_handlers.LikePostHandler)       //
	secured.POST("/like/post/get", like_handlers.GetPostLikesHandler)   //
	secured.POST("/like/comment/set", like_handlers.LikeCommentHandler) //

	secured.POST("/comment/get", comment_handlers.GetCommentsHandler)        //
	secured.POST("/comment/set", comment_handlers.CreateCommentHandler)      //
	secured.PUT("/comment/update", comment_handlers.UpdateCommentHandler)    //
	secured.DELETE("/comment/delete", comment_handlers.DeleteCommentHandler) //

	secured.POST("/follow/set", relation_handlers.FollowHandler)        //
	secured.DELETE("/follow/delete", relation_handlers.UnFollowHandler) //
	secured.POST("/friend/set", relation_handlers.FriendHandler)        //
	secured.DELETE("/friend/delete", relation_handlers.UnFriendHandler) //
	secured.POST("/block/set", relation_handlers.BlockHandler)          //
	secured.DELETE("/block/delete", relation_handlers.UnBlockHandler)   //

	secured.POST("/saved/get", saved_handlers.GetSavedPostsHandler)   //
	secured.POST("/saved/set", saved_handlers.SavePostHandler)        //
	secured.DELETE("/saved/delete", saved_handlers.UnsavePostHandler) //

	// --- Reglage ---
	secured.PUT("/settings/profile/update", auth_handlers.UpdateProfileHandler)                        //
	secured.PATCH("/settings/privacy/update", user_settings_handlers.UpdatePrivacyHandler)             //
	secured.PATCH("/settings/notifications/update", user_settings_handlers.UpdateNotificationsHandler) //
	secured.PATCH("/settings/display/update", user_settings_handlers.UpdateDisplayHandler)             //

	// --- Administration / Modération ---
	secured.POST("/ban", BanHandler)                                            // ℹ️❌
	secured.POST("/restriction", RestrictionHandler)                            // ℹ️❌
	secured.POST("/warning", WarningHandler)                                    // ℹ️❌
	secured.GET("/reports", LoadReportHandler)                                  // ℹ️❌
	secured.DELETE("/report", CloseReportHandler)                               // ℹ️❌
	secured.PUT("/report", UpdateManagerReportHandler)                          // ℹ️❌
	secured.GET("/information-user", LoadAdminInformationUserHandler)           // ℹ️❌
	secured.GET("/information-group", LoadAdminInformationGroupHandler)         // ℹ️❌
	secured.GET("/information-community", LoadAdminInformationCommunityHandler) // ℹ️❌
	secured.GET("/information-post", LoadAdminInformationPostHandler)           // ℹ️❌
	secured.GET("/information-comment", LoadAdminInformationCommentHandler)     // ℹ️❌
	secured.GET("/information-message", LoadAdminInformationMessageHandler)     // ℹ️❌

	// --- Messagerie / Groupes ---
	secured.POST("/conversations/user/get", conversation_handlers.GetUserConversationsHandler)                           //
	secured.POST("/conversations/get", conversation_handlers.GetUserConversationsHandler)                                //
	secured.POST("/conversation/members/get", conversation_handlers.GetConversationMembersHandler)                       //
	secured.POST("/conversation/suggest", conversation_handlers.SuggestContactsHandler)                                  //
	secured.POST("/conversation/set", conversation_handlers.CreateConversationHandler)                                   //
	secured.POST("/conversation/community/set", conversation_handlers.CreateCommunityHandler)                            //
	secured.POST("conversation/community/members/requests/get", conversation_handlers.GetCommunityRequestsHandler)       //
	secured.POST("conversation/community/members/requests/accept", conversation_handlers.AcceptCommunityRequestHandler)  //
	secured.POST("conversation/community/members/requests/refusal", conversation_handlers.RefuseCommunityRequestHandler) //
	secured.PUT("/conversation/update", conversation_handlers.UpdateConversationHandler)                                 //
	secured.PATCH("/conversation/settings", conversation_handlers.UpdateMemberSettingsHandler)                           //
	secured.POST("/conversation/pin", conversation_handlers.PinConversationHandler)                                      //
	secured.DELETE("/conversation/unpin", conversation_handlers.UnpinConversationHandler)                                //
	secured.DELETE("/conversation/delete", conversation_handlers.LeaveConversationHandler)                               //
	secured.POST("/messages/get", message_handlers.GetMessagesHandler)                                                   //
	secured.POST("messages/reactions/list", message_handlers.GetMessageReactionsHandler)                                 //

	secured.POST("/group/user/set", conversation_handlers.AddMemberHandler)            //
	secured.DELETE("/group/user/delete", conversation_handlers.BanMemberHandler)       //
	secured.POST("/group/promote/set", conversation_handlers.PromoteMemberHandler)     //
	secured.DELETE("/group/promote/delete", conversation_handlers.DemoteMemberHandler) //
	secured.POST("/group/join", conversation_handlers.JoinGroupHandler)                //

	// --- Media ---
	secured.POST("/media/upload", media_handlers.UploadMediaHandler) //
	secured.POST("/media/sign", media_handlers.SignMediaHandler)     //

	// --- Recherche ---
	secured.POST("/search/autocomplete/text", search_handlers.AutocompleteTextHandler) //
	secured.POST("/search/autocomplete/tags", search_handlers.AutocompleteTagHandler)  //
	secured.POST("/search/post", search_handlers.SearchPostHandler)                    //

	// --- Report ---
	secured.POST("/report", report_handlers.CreateReportHandler) //
}

func BanHandler(c *gin.Context) {
	// TODO: gérer les bans
	c.JSON(http.StatusOK, gin.H{"message": "user banned"})
}

func RestrictionHandler(c *gin.Context) {
	// TODO: gérer les restrictions
	c.JSON(http.StatusOK, gin.H{"message": "user restricted"})
}

func WarningHandler(c *gin.Context) {
	// TODO: gérer les avertissements
	c.JSON(http.StatusOK, gin.H{"message": "user warned"})
}

func LoadReportHandler(c *gin.Context) {
	// TODO: charger les rapports depuis la base
	c.JSON(http.StatusOK, gin.H{"reports": []string{"reports 1", "reports 2"}})
}

func CloseReportHandler(c *gin.Context) {
	// TODO: fermer un rapport
	c.JSON(http.StatusOK, gin.H{"message": "report closed"})
}

func UpdateManagerReportHandler(c *gin.Context) {
	// TODO: gérer la mise à jour du manager d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "update manager report"})
}

func LoadAdminInformationUserHandler(c *gin.Context) {
	// TODO: charger les informations d'un utilisateur
	c.JSON(http.StatusOK, gin.H{"message": "information user"})
}

func LoadAdminInformationGroupHandler(c *gin.Context) {
	// TODO: charger les informations d'un groupe
	c.JSON(http.StatusOK, gin.H{"message": "information group"})
}

func LoadAdminInformationCommunityHandler(c *gin.Context) {
	// TODO: charger les informations d'une communauté
	c.JSON(http.StatusOK, gin.H{"message": "information community"})
}

func LoadAdminInformationPostHandler(c *gin.Context) {
	// TODO: charger les informations d'un post_service
	c.JSON(http.StatusOK, gin.H{"message": "information post_service"})
}

func LoadAdminInformationCommentHandler(c *gin.Context) {
	// TODO: charger les informations d'un commentaire
	c.JSON(http.StatusOK, gin.H{"message": "information comment"})
}

func LoadAdminInformationMessageHandler(c *gin.Context) {
	// TODO: charger les informations d'un message
	c.JSON(http.StatusOK, gin.H{"message": "information message"})
}
