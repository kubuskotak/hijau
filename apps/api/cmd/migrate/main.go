// Command migrate applies database migrations. Usage:
//
//	go run ./cmd/migrate [up|down|status|reset|version]   (default: up)
package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"

	api "github.com/portierglobal/hijau/apps/api"
	"github.com/portierglobal/hijau/apps/api/internal/config"
)

func main() {
	config.LoadDotenv()

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := api.RunGoose(context.Background(), db, command, os.Args[2:]...); err != nil {
		log.Fatalf("migrate %s: %v", command, err)
	}
	log.Printf("migrate %s: ok", command)
}
