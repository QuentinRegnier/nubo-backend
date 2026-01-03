package docs

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
)

func InitDocsRoutes(r *gin.Engine) {
	// Swagger UI
	r.GET("/swagger.json", func(c *gin.Context) {
		c.File("./docs/swagger.json")
	})
	// Swagger JSON
	r.StaticFile("/docs/swagger.json", "/app/docs/swagger.json")

	// Redoc HTML
	r.StaticFile("/docs", "/app/docs/redoc.html")

	// Articles Markdown (LA BONNE FAÇON)
	r.Static("/docs/articles", "./docs/articles")

	r.GET("/docs/html/:filename", func(c *gin.Context) {
		filename := c.Param("filename")
		// Sécurité basique
		if filename == "" || filename == "." || filename == ".." {
			c.String(400, "Nom de fichier invalide")
			return
		}

		path := "./docs/articles/" + filename
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Erreur lecture fichier %s : %v", path, err)
			c.String(404, "Fichier non trouvé")
			return
		}

		// Conversion Markdown -> HTML brut
		htmlContent := markdown.ToHTML(data, nil, nil)

		// ON AJOUTE DU STYLE CSS "SCOPÉ" (ISOLÉ)
		// On n'utilise plus 'body' pour éviter de casser le layout global
		const style = `
		<style>
			/* On cible uniquement le conteneur de l'article */
			.markdown-body {
				color: #e5e7eb;            /* Texte gris clair */
				font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
				/* padding: 20px;  <-- SUPPRIMÉ : C'était ça qui décalait tout ! */
				line-height: 1.6;
				font-size: 1rem;
			}
			
			/* On préfixe tout par .markdown-body pour ne pas toucher au menu */
			.markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4 { 
				color: #60a5fa; 
				margin-top: 1.5em; 
				margin-bottom: 0.5em;
			} 
			
			.markdown-body a { color: #93c5fd; text-decoration: none; }
			.markdown-body a:hover { text-decoration: underline; }
			
			.markdown-body code {
				background-color: #1f2937;
				color: #e5e7eb;
				padding: 2px 6px;
				border-radius: 4px;
				font-family: monospace;
				font-size: 0.9em;
			}
			
			.markdown-body pre {
				background-color: #111827;
				padding: 15px;
				border-radius: 8px;
				overflow-x: auto;
				border: 1px solid #374151;
				margin: 1em 0;
			}
			
			.markdown-body ul, .markdown-body ol { margin-left: 20px; }
			
			.markdown-body blockquote {
				border-left: 4px solid #60a5fa;
				margin: 1em 0;
				padding-left: 15px;
				color: #9ca3af;
			}
		</style>
		`

		// IMPORTANT : On encapsule le HTML dans une div avec la classe .markdown-body
		finalHTML := style + `<div class="markdown-body">` + string(htmlContent) + `</div>`

		c.Data(200, "text/html; charset=utf-8", []byte(finalHTML))
	})
}
