package main

import (
	"fmt"
	"log"
	"os"

	"github.com/QuentinRegnier/nubo-backend/admin/service"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("❌ Usage: go run cmd/admin/main.go <image.avif>")
		os.Exit(1)
	}

	// Lecture du fichier en RAM (Respect de la règle Zero Disk après lecture)
	fileData, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("❌ Erreur lecture fichier : %v", err)
	}

	fmt.Println("🔍 Analyse forensique en cours (Algorithme DCT)...")

	// L'appel utilise bien le fichier forensic_service.go !
	report, err := service.ExtractForensicData(fileData)
	if err != nil {
		log.Fatalf("❌ Échec de l'extraction : %v", err)
	}

	displayReport(report)
}

func displayReport(r *service.WatermarkReport) {
	fmt.Println("\n==================================================")
	fmt.Println("🚨 RÉSULTAT DE L'INVESTIGATION")
	fmt.Println("==================================================")
	fmt.Printf("📸 Auteur Original : %s\n", r.OriginalAuthorID)
	fmt.Printf("📝 ID du Post      : %s\n", r.PostID)
	fmt.Printf("🕵️  COUPABLE (User) : %s\n", r.CulpritID)
	fmt.Printf("⏰ Date du vol     : %s\n", r.LeakTimestamp.Format("02/01/2006 à 15:04:05"))
	fmt.Println("==================================================")
}
