package api

import (
	"net/http"
	"os"
	"time"

	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/auth_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/comment_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/like_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/post_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/report_handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers/security_handlers"
	"github.com/golang-jwt/jwt/v5"

	"github.com/QuentinRegnier/nubo-backend/internal/api/handlers"
	"github.com/QuentinRegnier/nubo-backend/internal/api/middleware"
	"github.com/QuentinRegnier/nubo-backend/internal/api/websocket"
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
	r.POST("/signup", auth_handlers.SignUpHandler)
	r.POST("/login", auth_handlers.LoginHandler)

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

	// WebSocket Connection (Sécu via Query param ou Header standard, géré par le handler WS)
	// Note: Si tu veux JWT pour le WS, tu peux utiliser le middleware ici ou dans le handler.
	// Pour l'instant, je le laisse avec le middleware JWT comme dans ton exemple.
	r.GET("/ws", middleware.JWTMiddleware(), websocket.WSHandler)

	// =========================================================================
	// 2. ROUTES SÉCURISÉES (JWT + HMAC + RATCHET)
	// Toutes les routes ci-dessous nécessitent :
	// - Un JWT valide (Identity)
	// - Une signature HMAC valide (Integrity & Anti-Replay)
	// =========================================================================

	// On crée un groupe "plat" qui applique les deux middlewares d'un coup
	secured := r.Group("/")
	secured.Use(middleware.JWTMiddleware())  // 1. Qui est-ce ? (Populate context with UserID & DeviceToken)
	secured.Use(middleware.HMACMiddleware()) // 2. Est-ce authentique ? (Check Signature with Redis Secret)

	// --- Posts ---
	secured.GET("/feed", LoadFeedHandler)              // ℹ️❌
	secured.GET("/feed/more", LoadMoreFeedHandler)     // ℹ️❌
	secured.GET("/post", post_handlers.GetPostHandler) // ℹ️❌
	secured.POST("/post", post_handlers.CreatePostHandler)
	secured.PATCH("/post", post_handlers.UpdatePostHandler)
	secured.DELETE("/post", post_handlers.DeletePostHandler)
	secured.POST("/views/batch", handlers.RegisterBatchViewsHandler) // ℹ️❌ à vérifier
	secured.GET("/post/user", post_handlers.GetUserPostsHandler)
	secured.GET("/post/user/force", post_handlers.GetUserPostsHandler) // ✅ Cible exactement le même handler

	// --- Profils / Utilisateurs ---
	secured.GET("/search/users/quick", handlers.UserSearchHandler) // ℹ️❌ à vérifier

	// --- Actions Sociales ---
	secured.POST("/like/post", like_handlers.LikePostHandler)
	secured.GET("/like/post", like_handlers.GetPostLikesHandler)
	secured.POST("/like/comment", like_handlers.LikeCommentHandler)
	secured.POST("/comment", comment_handlers.CreateCommentHandler)
	secured.PATCH("/comment", comment_handlers.UpdateCommentHandler)
	secured.DELETE("/comment", comment_handlers.DeleteCommentHandler)
	secured.GET("/comment", comment_handlers.GetCommentsHandler)
	secured.POST("/follow", FollowHandler)     // ℹ️❌
	secured.DELETE("/follow", UnFollowHandler) // ℹ️❌
	secured.POST("/friend", FriendHandler)     // ℹ️❌
	secured.DELETE("/friend", UnFriendHandler) // ℹ️❌
	secured.POST("/block", BlockHandler)       // ℹ️❌
	secured.DELETE("/block", UnBlockHandler)   // ℹ️❌
	secured.POST("/share", ShareHandler)       // ℹ️❌
	secured.POST("/save", SaveHandler)         // ℹ️❌
	secured.DELETE("/saved", UnSavedHandler)   // ℹ️❌
	secured.GET("/saveds", LoadSavedsHandler)  // ℹ️❌

	// --- Reglage ---
	secured.PATCH("/profile", UpdateProfileHangler)            // ℹ️❌
	secured.PATCH("/confidentials", UpdateConfidentialHandler) // ℹ️❌
	secured.POST("/logout", LogoutHandler)                     // ℹ️❌
	secured.GET("/sessions", LoadSessionsHandler)              // ℹ️❌
	secured.DELETE("/sessions", DeleteSessionsHandler)         // ℹ️❌
	secured.PATCH("/language", UpdateLanguageHandler)          // ℹ️❌

	// --- Administration / Modération ---
	secured.POST("/ban", BanHandler)                                            // ℹ️❌
	secured.POST("/restriction", RestrictionHandler)                            // ℹ️❌
	secured.POST("/warning", WarningHandler)                                    // ℹ️❌
	secured.GET("/reports", LoadReportHandler)                                  // ℹ️❌
	secured.DELETE("/report", CloseReportHandler)                               // ℹ️❌
	secured.PATCH("/report", UpdateManagerReportHandler)                        // ℹ️❌
	secured.GET("/information-user", LoadAdminInformationUserHandler)           // ℹ️❌
	secured.GET("/information-group", LoadAdminInformationGroupHandler)         // ℹ️❌
	secured.GET("/information-community", LoadAdminInformationCommunityHandler) // ℹ️❌
	secured.GET("/information-post", LoadAdminInformationPostHandler)           // ℹ️❌
	secured.GET("/information-comment", LoadAdminInformationCommentHandler)     // ℹ️❌
	secured.GET("/information-message", LoadAdminInformationMessageHandler)     // ℹ️❌

	// --- Messagerie / Groupes ---
	secured.GET("/inbox", handlers.InboxHandler)                       // <--- SPEED Cache: Démarrage Inbox
	secured.POST("/conversation", ConversationHandler)                 // ℹ️❌
	secured.DELETE("/conversation", DeleteConversationHandler)         // ℹ️❌
	secured.PATCH("/conversation", ModifyConversationHandler)          // ℹ️❌
	secured.GET("/conversations", LoadConversationHandler)             // ℹ️❌
	secured.POST("/message", MessageHandler)                           // ℹ️❌
	secured.GET("/messages", LoadNewMessagesHandler)                   // ℹ️❌
	secured.DELETE("/messages", DeleteMessagesHandler)                 // ℹ️❌
	secured.PATCH("/message", UpdateMessageHandler)                    // ℹ️❌
	secured.POST("/user-group", AddUserGroupHandler)                   // ℹ️❌
	secured.DELETE("/user-group", DeleteUserGroupHandler)              // ℹ️❌
	secured.POST("/promote-group", SetAdminGroupHandler)               // ℹ️❌
	secured.DELETE("/promote-group", DeleteAdminGroupHandler)          // ℹ️❌
	secured.GET("/images-conversation", LoadImagesConversationHandler) // ℹ️❌
	secured.GET("/community", LoadCommunityHandler)                    // ℹ️❌
	secured.PATCH("/community", UpdateCommunityHandler)                // ℹ️❌
	secured.POST("/community", CreateCommunityHandler)                 // ℹ️❌
	secured.DELETE("/community", DeleteCommunityHandler)               // ℹ️❌
	secured.POST("/join", JoinGroupHandler)                            // ℹ️❌
	secured.POST("/quit", QuitGroupHandler)                            // ℹ️❌

	// --- Recherche ---
	secured.POST("/search/user", SearchUserHandler)           // ℹ️❌
	secured.POST("/search/post", SearchPostHandler)           // ℹ️❌
	secured.POST("/search/community", SearchCommunityHandler) // ℹ️❌
	secured.POST("/search/message", SearchMessageHandler)     // ℹ️❌
	secured.POST("/search/group", SearchGroupHandler)         // ℹ️❌
	secured.POST("/search/tag", SearchTagHandler)             // ℹ️❌

	// --- Report ---
	secured.POST("/report", report_handlers.CreateReportHandler)
}

func LoadFeedHandler(c *gin.Context) {
	// TODO: charger les posts du feed_service
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post_service 5", "post_service 6"}})
}

func LoadMoreFeedHandler(c *gin.Context) {
	// TODO: charger plus de posts depuis la base
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post_service 3", "post_service 4"}})
}

func FollowHandler(c *gin.Context) {
	// TODO: gérer les relations d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship managed"})
}

func UnFollowHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship removed"})
}

func FriendHandler(c *gin.Context) {
	// TODO: gérer les relations d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship managed"})
}

func UnFriendHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship removed"})
}

func BlockHandler(c *gin.Context) {
	// TODO: gérer les relations d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship managed"})
}

func UnBlockHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship removed"})
}

func ShareHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post_service shared"})
}

func SaveHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post_service saved"})
}

func UnSavedHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post_service saved deleted"})
}

func LoadSavedsHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post_service saved load"})
}

func UpdateProfileHangler(c *gin.Context) {
	// TODO: gérer la mise à jour du profil
	c.JSON(http.StatusOK, gin.H{"message": "profile updated"})
}

func UpdateConfidentialHandler(c *gin.Context) {
	// TODO: gérer la mise à jour des informations confidentielles
	c.JSON(http.StatusOK, gin.H{"message": "confidential updated"})
}

func LogoutHandler(c *gin.Context) {
	// TODO: gérer la déconnexion
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func LoadSessionsHandler(c *gin.Context) {
	// TODO: charger les sessions depuis la base
	c.JSON(http.StatusOK, gin.H{"sessions": []string{"session 1", "session 2"}})
}

func DeleteSessionsHandler(c *gin.Context) {
	// TODO: supprimer les sessions de la base
	c.JSON(http.StatusOK, gin.H{"message": "sessions deleted"})
}

func UpdateLanguageHandler(c *gin.Context) {
	// TODO: gérer la mise à jour de la langue
	c.JSON(http.StatusOK, gin.H{"message": "language updated"})
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

func ConversationHandler(c *gin.Context) {
	// TODO: gérer la création d'une nouvelle conversation
	c.JSON(http.StatusOK, gin.H{"message": "new conversation created"})
}

func DeleteConversationHandler(c *gin.Context) {
	// TODO: supprimer une conversation
	c.JSON(http.StatusOK, gin.H{"message": "conversation deleted"})
}

func ModifyConversationHandler(c *gin.Context) {
	// TODO: gérer la modification d'une conversation
	c.JSON(http.StatusOK, gin.H{"message": "conversation updated"})
}

func LoadConversationHandler(c *gin.Context) {
	// TODO: charger les conversations depuis la base
	c.JSON(http.StatusOK, gin.H{"conversations": []string{"conversation 1", "conversation 2", "conversation 3"}})
}

func MessageHandler(c *gin.Context) {
	// TODO: gérer l'envoi d'un message
	c.JSON(http.StatusOK, gin.H{"message": "message sent"})
}

func LoadNewMessagesHandler(c *gin.Context) {
	// TODO: gérer le chargement des nouveaux messages
	c.JSON(http.StatusOK, gin.H{"messages": []string{"new message 1", "new message 2"}})
}

func DeleteMessagesHandler(c *gin.Context) {
	// TODO: gérer la suppression des messages
	c.JSON(http.StatusOK, gin.H{"message": "messages deleted"})
}

func UpdateMessageHandler(c *gin.Context) {
	// TODO: gérer la mise à jour d'un message
	c.JSON(http.StatusOK, gin.H{"message": "message updated"})
}

func AddUserGroupHandler(c *gin.Context) {
	// TODO: gérer l'ajout d'un utilisateur à un groupe
	c.JSON(http.StatusOK, gin.H{"message": "user added to group"})
}

func DeleteUserGroupHandler(c *gin.Context) {
	// TODO: gérer la suppression d'un utilisateur d'un groupe
	c.JSON(http.StatusOK, gin.H{"message": "user deleted from group"})
}

func SetAdminGroupHandler(c *gin.Context) {
	// TODO: gérer la mise en admin d'un utilisateur
	c.JSON(http.StatusOK, gin.H{"message": "user promoted to admin"})
}

func DeleteAdminGroupHandler(c *gin.Context) {
	// TODO: gérer la suppression de l'admin d'un utilisateur
	c.JSON(http.StatusOK, gin.H{"message": "admin deleted from group"})
}

func LoadImagesConversationHandler(c *gin.Context) {
	// TODO: charger les images des conversations depuis la base
	c.JSON(http.StatusOK, gin.H{"images": []string{"image 1", "image 2"}})
}

func LoadCommunityHandler(c *gin.Context) {
	// TODO: charger les communautés depuis la base
	c.JSON(http.StatusOK, gin.H{"communities": []string{"community 1", "community 2"}})
}

func UpdateCommunityHandler(c *gin.Context) {
	// TODO: gérer la mise à jour d'une communauté
	c.JSON(http.StatusOK, gin.H{"message": "community updated"})
}

func CreateCommunityHandler(c *gin.Context) {
	// TODO: gérer la création d'une nouvelle communauté
	c.JSON(http.StatusOK, gin.H{"message": "community created"})
}

func DeleteCommunityHandler(c *gin.Context) {
	// TODO: gérer la suppression d'une communauté
	c.JSON(http.StatusOK, gin.H{"message": "community deleted"})
}

func JoinGroupHandler(c *gin.Context) {
	// TODO: gérer l'ajout d'un utilisateur à un groupe
	c.JSON(http.StatusOK, gin.H{"message": "group joined"})
}

func QuitGroupHandler(c *gin.Context) {
	// TODO: gérer la sortie d'un utilisateur d'un groupe
	c.JSON(http.StatusOK, gin.H{"message": "group left"})
}

func SearchUserHandler(c *gin.Context) {
	// TODO: gérer la recherche d'un utilisateur
	c.JSON(http.StatusOK, gin.H{"users": []string{"user 1", "user 2"}})
}

func SearchPostHandler(c *gin.Context) {
	// TODO: gérer la recherche d'un post_service
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post_service 1", "post_service 2"}})
}

func SearchCommunityHandler(c *gin.Context) {
	// TODO: gérer la recherche d'une communauté
	c.JSON(http.StatusOK, gin.H{"communities": []string{"community 1", "community 2"}})
}

func SearchMessageHandler(c *gin.Context) {
	// TODO: gérer la recherche d'un message
	c.JSON(http.StatusOK, gin.H{"messages": []string{"new message 1", "new message 2"}})
}

func SearchGroupHandler(c *gin.Context) {
	// TODO: gérer la recherche d'un groupe
	c.JSON(http.StatusOK, gin.H{"groups": []string{"group 1", "group 2"}})
}

func SearchTagHandler(c *gin.Context) {
	// TODO: gérer la recherche d'un tag
	c.JSON(http.StatusOK, gin.H{"tags": []string{"tag 1", "tag 2"}})
}
