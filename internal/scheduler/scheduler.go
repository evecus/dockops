package scheduler

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dockops/dockops/internal/db"
	"github.com/dockops/dockops/internal/docker"
	"github.com/robfig/cron/v3"
)

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

// ImageUpdateStatus stores per-tag update check results.
type ImageUpdateStatus struct {
	Tag         string    `json:"tag"`
	HasUpdate   bool      `json:"has_update"`
	CheckedAt   time.Time `json:"checked_at"`
}

type imageUpdateCache struct {
	mu     sync.RWMutex
	status map[string]ImageUpdateStatus // key = tag
}

func (c *imageUpdateCache) set(tag string, hasUpdate bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status[tag] = ImageUpdateStatus{Tag: tag, HasUpdate: hasUpdate, CheckedAt: time.Now()}
}

func (c *imageUpdateCache) get(tag string) (ImageUpdateStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.status[tag]
	return s, ok
}

func (c *imageUpdateCache) all() map[string]ImageUpdateStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]ImageUpdateStatus, len(c.status))
	for k, v := range c.status {
		out[k] = v
	}
	return out
}

type Scheduler struct {
	db               *db.DB
	cron             *cron.Cron
	collectEntryID   cron.EntryID
	imgCheckEntryID  cron.EntryID
	Cache            *DashboardCache
	imgCache         *imageUpdateCache
}

func New(database *db.DB) *Scheduler {
	return &Scheduler{
		db:    database,
		cron:  cron.New(),
		Cache: &DashboardCache{},
		imgCache: &imageUpdateCache{status: make(map[string]ImageUpdateStatus)},
	}
}

func (s *Scheduler) Start() {
	collectInterval, err := s.db.GetSetting("collect_interval")
	if err != nil || collectInterval == "" {
		collectInterval = "10m"
	}
	if collectInterval != "off" {
		s.scheduleCollect(collectInterval)
	}

	imgInterval, err := s.db.GetSetting("update_check_interval")
	if err != nil || imgInterval == "" {
		imgInterval = "6h"
	}
	if imgInterval != "off" {
		s.scheduleImageCheck(imgInterval)
	}

	s.cron.Start()
	go s.collectDashboard()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
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

func (s *Scheduler) UpdateImageCheckInterval(interval string) error {
	if err := s.db.SetSetting("update_check_interval", interval); err != nil {
		return err
	}
	if s.imgCheckEntryID != 0 {
		s.cron.Remove(s.imgCheckEntryID)
		s.imgCheckEntryID = 0
	}
	if interval != "off" {
		s.scheduleImageCheck(interval)
	}
	return nil
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

func (s *Scheduler) scheduleImageCheck(interval string) {
	spec := imageCheckIntervalToSpec(interval)
	id, err := s.cron.AddFunc(spec, func() { s.checkImageUpdates() })
	if err != nil {
		log.Printf("Failed to schedule image update check: %v", err)
		return
	}
	s.imgCheckEntryID = id
}

func (s *Scheduler) CollectNow() {
	go s.collectDashboard()
}

func (s *Scheduler) CheckImageUpdatesNow() {
	go s.checkImageUpdates()
}

// GetImageUpdateStatus returns update status for all known tags.
func (s *Scheduler) GetImageUpdateStatus() map[string]ImageUpdateStatus {
	return s.imgCache.all()
}

// MarkImageUpdated clears the "has_update" flag after a manual update.
func (s *Scheduler) MarkImageUpdated(tag string) {
	s.imgCache.set(tag, false)
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
}

// checkImageUpdates checks for updates only on images currently used by containers.
func (s *Scheduler) checkImageUpdates() {
	client, err := docker.NewClient()
	if err != nil {
		log.Printf("Image update check: docker client error: %v", err)
		return
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Only check tags that are actually in use by a container
	containers, err := client.ListContainers(ctx)
	if err != nil {
		log.Printf("Image update check: list containers error: %v", err)
		return
	}

	seen := make(map[string]bool)
	for _, ct := range containers {
		tag := ct.Image
		if tag == "" || tag == "<none>:<none>" || seen[tag] {
			continue
		}
		seen[tag] = true

		hasUpdate, err := checkTagHasUpdate(ctx, client, tag)
		if err != nil {
			log.Printf("Image update check [%s]: %v", tag, err)
			continue
		}
		s.imgCache.set(tag, hasUpdate)
	}
}

// checkTagHasUpdate checks remote digest vs local digest using docker pull --dry-run
// equivalent: DistributionInspect to get remote digest, compare with local RepoDigests.
func checkTagHasUpdate(ctx context.Context, c *docker.Client, tag string) (bool, error) {
	localID, err := c.GetImageID(ctx, tag)
	if err != nil {
		return false, err
	}

	// Pull only the manifest (no layers) to get remote digest
	remoteDigest, err := c.GetRemoteDigest(ctx, tag)
	if err != nil {
		// Network error or private registry without auth — skip silently
		return false, nil
	}

	localDigests, err := c.GetLocalDigests(ctx, tag)
	if err != nil {
		return false, err
	}

	_ = localID

	for _, d := range localDigests {
		if strings.Contains(d, remoteDigest) || d == remoteDigest {
			return false, nil
		}
	}
	return true, nil
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

func imageCheckIntervalToSpec(d string) string {
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


