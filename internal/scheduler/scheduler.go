package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/dockops/dockops/internal/compose"
	"github.com/dockops/dockops/internal/db"
	"github.com/dockops/dockops/internal/docker"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	db      *db.DB
	cron    *cron.Cron
	entryID cron.EntryID
}

func New(database *db.DB) *Scheduler {
	return &Scheduler{
		db:   database,
		cron: cron.New(),
	}
}

func (s *Scheduler) Start() {
	interval, err := s.db.GetSetting("update_check_interval")
	if err != nil || interval == "" {
		interval = "6h"
	}
	s.scheduleCheck(interval)
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) UpdateInterval(interval string) error {
	if err := s.db.SetSetting("update_check_interval", interval); err != nil {
		return err
	}
	if s.entryID != 0 {
		s.cron.Remove(s.entryID)
	}
	s.scheduleCheck(interval)
	return nil
}

func (s *Scheduler) scheduleCheck(interval string) {
	spec := durationToSpec(interval)
	id, err := s.cron.AddFunc(spec, func() {
		s.checkUpdates()
	})
	if err != nil {
		log.Printf("Failed to schedule update check: %v", err)
		return
	}
	s.entryID = id
}

func (s *Scheduler) CheckNow() {
	go s.checkUpdates()
}

func (s *Scheduler) checkUpdates() {
	log.Println("Checking for container image updates...")
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Printf("Failed to create docker client for update check: %v", err)
		return
	}
	defer dockerClient.Close()

	mgr := compose.NewManager(s.db)
	containers, err := mgr.GetAllForUpdateCheck()
	if err != nil {
		log.Printf("Failed to list containers for update check: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	for _, ct := range containers {
		images := compose.ExtractImageFromCompose(ct.ComposeContent)
		for _, img := range images {
			localID, err := dockerClient.GetImageID(ctx, img)
			if err != nil {
				continue
			}

			reader, err := dockerClient.StreamPullImage(ctx, img)
			if err != nil {
				continue
			}

			// consume pull output
			buf := make([]byte, 4096)
			for {
				_, err := reader.Read(buf)
				if err != nil {
					break
				}
			}
			reader.Close()

			remoteID, err := dockerClient.GetImageID(ctx, img)
			if err != nil {
				continue
			}

			mgr.SetUpdateAvailable(ct.ID, remoteID != localID)
		}
	}
	log.Println("Update check complete")
}

func durationToSpec(d string) string {
	switch d {
	case "1h":
		return "0 * * * *"
	case "6h":
		return "0 */6 * * *"
	case "12h":
		return "0 */12 * * *"
	case "24h":
		return "0 0 * * *"
	default:
		return "0 */6 * * *"
	}
}
