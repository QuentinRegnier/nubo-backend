package api

import (
	"net/http"
	"os"
	"time"

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

	// Active la gestion automatique des OPTIONS et du CORS
	r.Use(middleware.CORSMiddleware())

	// Récupération automatique des panics (évite que le serveur crash totalement)
	r.Use(gin.Recovery())

	// =========================================================================
	// 1. ROUTES PUBLIQUES (Aucune sécu ou sécu spécifique interne)
	// =========================================================================

	// Authentification (Sécu interne spécifique)
	r.POST("/signup", handlers.SignUpHandler)
	r.POST("/login", handlers.LoginHandler)

	// Renouvellement de Tokens (Ratchet / Master)
	// Ces routes gèrent leur propre sécurité (HMAC spécial, checks BDD...)
	r.POST("/renew-jwt", handlers.RenewJWT)
	r.POST("/refresh-master", handlers.RefreshMaster)

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
	secured.GET("/feed", LoadFeedHandler)          // ℹ️❌
	secured.GET("/feed/more", LoadMoreFeedHandler) // ℹ️❌
	secured.GET("/posts", GetPostsHandler)         // ℹ️❌
	secured.POST("/post", CreatePostHandler)       // ℹ️❌
	secured.PATCH("/post", ModifyPostHandler)      // ℹ️❌
	secured.DELETE("/post", DeletePost)            // ℹ️❌

	// --- Actions Sociales ---
	secured.POST("/like", LikeHandler)            // ℹ️❌
	secured.DELETE("/like", UnLikeHandler)        // ℹ️❌
	secured.POST("/comment", CommentHandler)      // ℹ️❌
	secured.DELETE("/comment", UnCommentHandler)  // ℹ️❌
	secured.GET("/comments", LoadCommentsHandler) // ℹ️❌
	secured.POST("/follow", FollowHandler)        // ℹ️❌
	secured.DELETE("/follow", UnFollowHandler)    // ℹ️❌
	secured.POST("/friend", FriendHandler)        // ℹ️❌
	secured.DELETE("/friend", UnFriendHandler)    // ℹ️❌
	secured.POST("/limited", LimitedHandler)      // ℹ️❌
	secured.DELETE("/limited", UnLimitedHandler)  // ℹ️❌
	secured.POST("/block", BlockHandler)          // ℹ️❌
	secured.DELETE("/block", UnBlockHandler)      // ℹ️❌
	secured.POST("/share", ShareHandler)          // ℹ️❌
	secured.POST("/save", SaveHandler)            // ℹ️❌
	secured.DELETE("/saved", UnSavedHandler)      // ℹ️❌
	secured.GET("/saveds", LoadSavedsHandler)     // ℹ️❌

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
	secured.POST("/report/user", ReportUserHandler)           // ℹ️❌
	secured.POST("/report/post", ReportPostHandler)           // ℹ️❌
	secured.POST("/report/comment", ReportCommentHandler)     // ℹ️❌
	secured.POST("/report/message", ReportMessageHandler)     // ℹ️❌
	secured.POST("/report/community", ReportCommunityHandler) // ℹ️❌
	secured.POST("/report/group", ReportGroupHandler)         // ℹ️❌
}

func LoadFeedHandler(c *gin.Context) {
	// TODO: charger les posts du feed
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 5", "post 6"}})
}

func LoadMoreFeedHandler(c *gin.Context) {
	// TODO: charger plus de posts depuis la base
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 3", "post 4"}})
}

func GetPostsHandler(c *gin.Context) {
	// TODO: récupérer les posts depuis la base
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 1", "post 2"}})
}

func CreatePostHandler(c *gin.Context) {
	// TODO: ajouter post à la base
	c.JSON(http.StatusCreated, gin.H{"message": "post created"})
}

func ModifyPostHandler(c *gin.Context) {
	// TODO: modifier un post dans la base
	c.JSON(http.StatusOK, gin.H{"message": "post modified"})
}

func DeletePost(c *gin.Context) {
	// TODO: supprimer post de la base
	c.JSON(http.StatusOK, gin.H{"message": "post deleted"})
}

func LikeHandler(c *gin.Context) {
	// TODO: ajouter un like à un post
	c.JSON(http.StatusOK, gin.H{"message": "post liked"})
}

func UnLikeHandler(c *gin.Context) {
	// TODO: retirer un like à un post
	c.JSON(http.StatusOK, gin.H{"message": "post unliked"})
}

func CommentHandler(c *gin.Context) {
	// TODO: ajouter un commentaire à un post
	c.JSON(http.StatusOK, gin.H{"message": "post commented"})
}

func UnCommentHandler(c *gin.Context) {
	// TODO: retirer un commentaire à un post
	c.JSON(http.StatusOK, gin.H{"message": "post uncommented"})
}

func LoadCommentsHandler(c *gin.Context) {
	// TODO: charger les commentaires d'un post depuis la base
	c.JSON(http.StatusOK, gin.H{"comments": []string{"comment 1", "comment 2"}})
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

func LimitedHandler(c *gin.Context) {
	// TODO: gérer les relations d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "friendship managed"})
}

func UnLimitedHandler(c *gin.Context) {
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
	c.JSON(http.StatusOK, gin.H{"message": "post shared"})
}

func SaveHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post saved"})
}

func UnSavedHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post saved deleted"})
}

func LoadSavedsHandler(c *gin.Context) {
	// TODO: retirer une relation d'amitié
	c.JSON(http.StatusOK, gin.H{"message": "post saved load"})
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
	// TODO: charger les informations d'un post
	c.JSON(http.StatusOK, gin.H{"message": "information post"})
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
	// TODO: gérer la recherche d'un post
	c.JSON(http.StatusOK, gin.H{"posts": []string{"post 1", "post 2"}})
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

func ReportUserHandler(c *gin.Context) {
	// TODO: gérer la création d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "user reported"})
}

func ReportPostHandler(c *gin.Context) {
	// TODO: gérer la création d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "post reported"})
}

func ReportCommentHandler(c *gin.Context) {
	// TODO: gérer la création d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "comment reported"})
}

func ReportMessageHandler(c *gin.Context) {
	// TODO: gérer la création d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "message reported"})
}

func ReportCommunityHandler(c *gin.Context) {
	// TODO: gérer la création d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "community reported"})
}

func ReportGroupHandler(c *gin.Context) {
	// TODO: gérer la création d'un rapport
	c.JSON(http.StatusOK, gin.H{"message": "group reported"})
}
