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
	configPath := flag.String("c", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	database, err := db.Init(cfg.DataPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	sched := scheduler.New(database, cfg.DataPath)
	sched.Start()

	srv := handler.NewServer(cfg, database, webFS, sched)
	if err := srv.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
