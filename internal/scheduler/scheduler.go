package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/dockops/dockops/internal/compose"
	"github.com/dockops/dockops/internal/db"
	"github.com/dockops/dockops/internal/docker"
	"github.com/robfig/cron/v3"
)

// DashboardCache holds the latest collected dashboard data.
type DashboardCache struct {
	Info      interface{}
	Stats     interface{}
	UpdatedAt time.Time
	mu        sync.RWMutex
}

func (c *DashboardCache) Set(info, stats interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Info = info
	c.Stats = stats
	c.UpdatedAt = time.Now()
}

func (c *DashboardCache) Get() (info, stats interface{}, updatedAt time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Info, c.Stats, c.UpdatedAt
}

type Scheduler struct {
	db             *db.DB
	dataPath       string
	cron           *cron.Cron
	updateEntryID  cron.EntryID
	collectEntryID cron.EntryID
	Cache          *DashboardCache
}

func New(database *db.DB, dataPath string) *Scheduler {
	return &Scheduler{
		db:       database,
		dataPath: dataPath,
		cron:     cron.New(),
		Cache:    &DashboardCache{},
	}
}

func (s *Scheduler) Start() {
	updateInterval, err := s.db.GetSetting("update_check_interval")
	if err != nil || updateInterval == "" {
		updateInterval = "6h"
	}
	if updateInterval != "off" {
		s.scheduleUpdateCheck(updateInterval)
	}

	collectInterval, err := s.db.GetSetting("collect_interval")
	if err != nil || collectInterval == "" {
		collectInterval = "10m"
	}
	if collectInterval != "off" {
		s.scheduleCollect(collectInterval)
	}

	s.cron.Start()
	go s.collectDashboard()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) UpdateInterval(interval string) error {
	if err := s.db.SetSetting("update_check_interval", interval); err != nil {
		return err
	}
	if s.updateEntryID != 0 {
		s.cron.Remove(s.updateEntryID)
		s.updateEntryID = 0
	}
	if interval != "off" {
		s.scheduleUpdateCheck(interval)
	}
	return nil
}

func (s *Scheduler) UpdateCollectInterval(interval string) error {
	if err := s.db.SetSetting("collect_interval", interval); err != nil {
		return err
	}
	if s.collectEntryID != 0 {
		s.cron.Remove(s.collectEntryID)
		s.collectEntryID = 0
	}
	if interval != "off" {
		s.scheduleCollect(interval)
	}
	return nil
}

func (s *Scheduler) scheduleUpdateCheck(interval string) {
	spec := updateIntervalToSpec(interval)
	id, err := s.cron.AddFunc(spec, func() { s.checkUpdates() })
	if err != nil {
		log.Printf("Failed to schedule update check: %v", err)
		return
	}
	s.updateEntryID = id
}

func (s *Scheduler) scheduleCollect(interval string) {
	spec := collectIntervalToSpec(interval)
	id, err := s.cron.AddFunc(spec, func() { s.collectDashboard() })
	if err != nil {
		log.Printf("Failed to schedule dashboard collect: %v", err)
		return
	}
	s.collectEntryID = id
}

func (s *Scheduler) CheckNow() {
	go s.checkUpdates()
}

func (s *Scheduler) CollectNow() {
	go s.collectDashboard()
}

func (s *Scheduler) collectDashboard() {
	client, err := docker.NewClient()
	if err != nil {
		log.Printf("Dashboard collect: docker client error: %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		log.Printf("Dashboard collect: get system info error: %v", err)
		return
	}

	containers, err := client.ListContainers(ctx)
	if err != nil {
		log.Printf("Dashboard collect: list containers error: %v", err)
		return
	}

	var totalCPU float64
	var totalMem, totalMemLimit uint64
	for _, ct := range containers {
		if ct.State == "running" {
			stats, err := client.GetContainerStats(ctx, ct.ID)
			if err == nil {
				totalCPU += stats.CPUPercent
				totalMem += stats.MemoryUsage
				totalMemLimit += stats.MemoryLimit
			}
		}
	}

	statsData := map[string]interface{}{
		"total_cpu_percent": totalCPU,
		"total_mem_usage":   totalMem,
		"total_mem_limit":   totalMemLimit,
		"containers":        len(containers),
	}

	s.Cache.Set(info, statsData)
	log.Println("Dashboard data collected and cached")
}

func (s *Scheduler) checkUpdates() {
	log.Println("Checking for container image updates...")
	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Printf("Failed to create docker client for update check: %v", err)
		return
	}
	defer dockerClient.Close()

	mgr := compose.NewManager(s.db, s.dataPath)
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

func updateIntervalToSpec(d string) string {
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

func collectIntervalToSpec(d string) string {
	switch d {
	case "1m":
		return "* * * * *"
	case "5m":
		return "*/5 * * * *"
	case "10m":
		return "*/10 * * * *"
	case "30m":
		return "*/30 * * * *"
	default:
		return "*/10 * * * *"
	}
}
