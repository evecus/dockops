package main

import (
	"embed"
	"flag"
	"log"

	"github.com/dockops/dockops/internal/config"
	"github.com/dockops/dockops/internal/db"
	"github.com/dockops/dockops/internal/handler"
	"github.com/dockops/dockops/internal/scheduler"
)

//go:embed all:web/dist
var webFS embed.FS

func main() {
	httpPort := flag.Int("http", 0, "HTTP port (default 9080)")
	httpsPort := flag.Int("https", 0, "HTTPS port (default 9443, only if certs exist)")
	dataDir := flag.String("dir", "", "data directory (default: ./data next to binary)")
	flag.Parse()

	cfg := config.New(*httpPort, *httpsPort, *dataDir)

	if err := cfg.Init(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	database, err := db.Init(cfg.DataPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	sched := scheduler.New(database)
	sched.Start()

	log.Printf("DockOps starting — HTTP :%d  data: %s", cfg.HTTPPort, cfg.DataPath)
	if cfg.CertPath != "" {
		log.Printf("HTTPS enabled — :%d", cfg.HTTPSPort)
	}

	srv := handler.NewServer(cfg, database, webFS, sched)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
