package handler

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/dockops/dockops/internal/auth"
	"github.com/dockops/dockops/internal/compose"
	"github.com/dockops/dockops/internal/config"
	"github.com/dockops/dockops/internal/db"
	"github.com/dockops/dockops/internal/docker"
	"github.com/dockops/dockops/internal/middleware"
	"github.com/dockops/dockops/internal/parser"
	"github.com/dockops/dockops/internal/scheduler"
	"github.com/dockops/dockops/internal/ws"
	"github.com/gin-gonic/gin"
)

type Server struct {
	cfg     *config.Config
	db      *db.DB
	webFS   embed.FS
	sched   *scheduler.Scheduler
	compose *compose.Manager
}

func NewServer(cfg *config.Config, database *db.DB, webFS embed.FS, sched *scheduler.Scheduler) *Server {
	return &Server{
		cfg:     cfg,
		db:      database,
		webFS:   webFS,
		sched:   sched,
		compose: compose.NewManager(database, cfg.DataPath),
	}
}

func ok(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": -1, "error": msg})
}

func (s *Server) Run() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		api.GET("/system/status", s.systemStatus)
		api.POST("/auth/setup", s.setup)
		api.POST("/auth/login", s.login)

		auth := api.Group("", middleware.Auth())
		{
			auth.GET("/dashboard/info", s.dashboardInfo)
			auth.GET("/dashboard/stats", s.dashboardStats)

			// Containers
			auth.GET("/containers", s.listContainers)
			auth.POST("/containers", s.createContainer)
			auth.POST("/containers/parse-run", s.parseDockerRun)
			auth.POST("/containers/check-updates", s.checkUpdates)
			auth.GET("/containers/:id", s.getContainer)
			auth.GET("/containers/:id/form-data", s.getContainerFormData)
			auth.PUT("/containers/:id", s.updateContainer)
			auth.DELETE("/containers/:id", s.deleteContainer)
			auth.POST("/containers/:id/start", s.startContainer)
			auth.POST("/containers/:id/stop", s.stopContainer)
			auth.POST("/containers/:id/restart", s.restartContainer)
			auth.GET("/containers/:id/stats", s.containerStats)
			auth.GET("/containers/:id/logs", s.containerLogs)
			auth.GET("/containers/:id/files", s.listFiles)
			auth.GET("/containers/:id/files/download", s.downloadFile)
			auth.POST("/containers/:id/files/upload", s.uploadFile)
			auth.DELETE("/containers/:id/files", s.deleteFile)
			auth.POST("/containers/:id/update", s.updateContainerImage)

			// Images
			auth.GET("/images", s.listImages)
			auth.POST("/images/pull", s.pullImage)
			auth.POST("/images/load", s.loadImage)
			auth.DELETE("/images/:id", s.deleteImage)

			// Networks
			auth.GET("/networks", s.listNetworks)
			auth.POST("/networks", s.createNetwork)
			auth.DELETE("/networks/:id", s.deleteNetwork)
			auth.POST("/networks/prune", s.pruneNetworks)

			// Volumes
			auth.GET("/volumes", s.listVolumes)
			auth.POST("/volumes", s.createVolume)
			auth.DELETE("/volumes/:name", s.deleteVolume)
			auth.POST("/volumes/prune", s.pruneVolumes)

			// Settings
			auth.GET("/settings", s.getSettings)
			auth.PUT("/settings", s.updateSettings)
			auth.POST("/settings/install-docker", s.installDocker)
			auth.PUT("/settings/admin", s.updateAdmin)
		}
	}

	// WebSocket
	r.GET("/ws/containers/:id/terminal", func(c *gin.Context) {
		ws.HandleTerminal(c.Writer, c.Request, c.Param("id"))
	})
	r.GET("/ws/containers/:id/logs", func(c *gin.Context) {
		ws.HandleLogs(c.Writer, c.Request, c.Param("id"))
	})

	// SPA frontend
	webSub, err := fs.Sub(s.webFS, "web/dist")
	if err != nil {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(404, gin.H{"error": "not found"})
			} else {
				c.String(200, "DockOps API Server")
			}
		})
	} else {
		fileServer := http.FileServer(http.FS(webSub))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/ws") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			if strings.HasPrefix(path, "/assets/") ||
				strings.HasSuffix(path, ".js") ||
				strings.HasSuffix(path, ".css") ||
				strings.HasSuffix(path, ".png") ||
				strings.HasSuffix(path, ".ico") ||
				strings.HasSuffix(path, ".svg") ||
				strings.HasSuffix(path, ".woff2") {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	errCh := make(chan error, 2)

	go func() {
		addr := fmt.Sprintf(":%d", s.cfg.HTTPPort)
		errCh <- r.Run(addr)
	}()

	if s.cfg.CertPath != "" && s.cfg.KeyPath != "" {
		go func() {
			addr := fmt.Sprintf(":%d", s.cfg.HTTPSPort)
			errCh <- r.RunTLS(addr, s.cfg.CertPath, s.cfg.KeyPath)
		}()
	}

	return <-errCh
}

// ===== AUTH =====

func (s *Server) systemStatus(c *gin.Context) {
	isSetup, _ := s.db.IsSetup()
	ok(c, gin.H{"setup": isSetup})
}

func (s *Server) setup(c *gin.Context) {
	isSetup, _ := s.db.IsSetup()
	if isSetup {
		fail(c, 400, "already setup")
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if err := auth.CreateAdmin(s.db, req.Username, req.Password); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "admin created"})
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	token, err := auth.Login(s.db, req.Username, req.Password)
	if err != nil {
		fail(c, 401, "invalid credentials")
		return
	}
	ok(c, gin.H{"token": token})
}

// ===== DASHBOARD =====

func (s *Server) dashboardInfo(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	info, err := client.GetSystemInfo(context.Background())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, info)
}

func (s *Server) dashboardStats(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	containers, err := client.ListContainers(context.Background())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	var totalCPU float64
	var totalMem, totalMemLimit uint64

	for _, ct := range containers {
		if ct.State == "running" {
			stats, err := client.GetContainerStats(context.Background(), ct.ID)
			if err == nil {
				totalCPU += stats.CPUPercent
				totalMem += stats.MemoryUsage
				totalMemLimit += stats.MemoryLimit
			}
		}
	}

	ok(c, gin.H{
		"total_cpu_percent": totalCPU,
		"total_mem_usage":   totalMem,
		"total_mem_limit":   totalMemLimit,
		"containers":        len(containers),
	})
}

// ===== CONTAINERS =====

// EnrichedContainer is the unified response shape for the containers list.
type EnrichedContainer struct {
	compose.ContainerRecord
	DockerState  string               `json:"docker_state"`
	DockerStatus string               `json:"docker_status"`
	Ports        []docker.PortBinding `json:"ports"`
}

func (s *Server) listContainers(c *gin.Context) {
	// 1. Get all real Docker containers
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	dockerContainers, err := client.ListContainers(context.Background())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	// 2. Build name→docker map
	dockerMap := make(map[string]docker.ContainerSummary)
	for _, dc := range dockerContainers {
		dockerMap[dc.Name] = dc
	}

	// 3. Load dockops DB records
	records, _ := s.compose.ListContainers()
	dbByName := make(map[string]compose.ContainerRecord)
	for _, r := range records {
		dbByName[r.Name] = r
	}

	// 4. Ensure every Docker container has a DB record (register externals)
	for _, dc := range dockerContainers {
		if _, exists := dbByName[dc.Name]; !exists {
			rec, err := s.compose.RegisterExternal(dc.Name)
			if err == nil {
				dbByName[dc.Name] = *rec
			}
		}
	}

	// 5. Reload DB records after registration
	records, _ = s.compose.ListContainers()

	// 6. Emit enriched list — only containers that actually exist in Docker
	var result []EnrichedContainer
	for _, r := range records {
		dc, exists := dockerMap[r.Name]
		if !exists {
			// Container is in DB but not in Docker (deleted externally) — still show it
			result = append(result, EnrichedContainer{ContainerRecord: r})
			continue
		}
		result = append(result, EnrichedContainer{
			ContainerRecord: r,
			DockerState:     dc.State,
			DockerStatus:    dc.Status,
			Ports:           dc.Ports,
		})
	}

	ok(c, result)
}

func (s *Server) createContainer(c *gin.Context) {
	var req compose.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	record, err := s.compose.CreateContainer(&req)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	if err := s.compose.Up(record.ID); err != nil {
		fail(c, 500, "Container created but failed to start: "+err.Error())
		return
	}

	ok(c, record)
}

func (s *Server) parseDockerRun(c *gin.Context) {
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	svc, err := parser.ParseDockerRun(req.Command)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}

	ok(c, gin.H{
		"service": svc,
		"yaml":    svc.ToYAML(),
	})
}

func (s *Server) getContainer(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "container not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		ok(c, record)
		return
	}
	defer client.Close()

	detail, _ := client.InspectContainer(context.Background(), record.Name)
	ok(c, gin.H{"record": record, "docker": detail})
}

// getContainerFormData returns pre-filled FormFields for the edit modal.
// For dockops containers it parses the stored compose YAML;
// for external containers it reverse-engineers docker inspect output.
func (s *Server) getContainerFormData(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "container not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	detail, err := client.InspectContainer(context.Background(), record.Name)
	if err != nil {
		fail(c, 500, "failed to inspect container: "+err.Error())
		return
	}

	// Build FormFields from docker inspect
	fields := &compose.FormFields{
		Image:      detail.Image,
		Hostname:   detail.Hostname,
		Privileged: false,
	}

	// Restart policy
	fields.Restart = detail.RestartPolicy

	// Ports: "hostPort:containerPort/proto"
	for _, p := range detail.Ports {
		if p.HostPort != "" && p.HostPort != "0" {
			fields.Ports = append(fields.Ports, fmt.Sprintf("%s:%s/%s", p.HostPort, p.ContainerPort, p.Protocol))
		}
	}

	// Volumes
	for _, m := range detail.Mounts {
		if m.Type == "bind" {
			fields.Volumes = append(fields.Volumes, fmt.Sprintf("%s:%s", m.Source, m.Destination))
		} else if m.Type == "volume" {
			fields.Volumes = append(fields.Volumes, fmt.Sprintf("%s:%s", m.Source, m.Destination))
		}
	}

	// Environment — filter out variables that have no value (likely image defaults)
	for _, e := range detail.Env {
		if strings.Contains(e, "=") {
			fields.Env = append(fields.Env, e)
		}
	}

	// Networks
	for net := range detail.Networks {
		if net != "bridge" && net != "host" && net != "none" {
			fields.NetworkMode = ""
		} else {
			fields.NetworkMode = net
		}
		break
	}

	ok(c, gin.H{
		"name":   record.Name,
		"source": record.Source,
		"fields": fields,
	})
}

func (s *Server) updateContainer(c *gin.Context) {
	id := c.Param("id")
	var req compose.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	if err := s.compose.UpdateContainer(id, &req); err != nil {
		fail(c, 500, err.Error())
		return
	}

	if err := s.compose.Up(id); err != nil {
		fail(c, 500, "Updated but failed to start: "+err.Error())
		return
	}

	ok(c, gin.H{"message": "updated"})
}

func (s *Server) deleteContainer(c *gin.Context) {
	id := c.Param("id")
	if err := s.compose.DeleteContainer(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "deleted"})
}

func (s *Server) startContainer(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	// External containers with no compose file: use docker start directly
	if record.Source == "external" || record.ComposeContent == "" {
		client, err := docker.NewClient()
		if err != nil {
			fail(c, 500, err.Error())
			return
		}
		defer client.Close()
		if err := client.StartContainer(context.Background(), record.Name); err != nil {
			fail(c, 500, err.Error())
			return
		}
		ok(c, gin.H{"message": "started"})
		return
	}

	if err := s.compose.Up(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "started"})
}

func (s *Server) stopContainer(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	if record.Source == "external" || record.ComposeContent == "" {
		client, err := docker.NewClient()
		if err != nil {
			fail(c, 500, err.Error())
			return
		}
		defer client.Close()
		if err := client.StopContainer(context.Background(), record.Name); err != nil {
			fail(c, 500, err.Error())
			return
		}
		ok(c, gin.H{"message": "stopped"})
		return
	}

	if err := s.compose.Down(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "stopped"})
}

func (s *Server) restartContainer(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	if record.Source == "external" || record.ComposeContent == "" {
		client, err := docker.NewClient()
		if err != nil {
			fail(c, 500, err.Error())
			return
		}
		defer client.Close()
		if err := client.RestartContainer(context.Background(), record.Name); err != nil {
			fail(c, 500, err.Error())
			return
		}
		ok(c, gin.H{"message": "restarted"})
		return
	}

	if err := s.compose.Down(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	time.Sleep(2 * time.Second)
	if err := s.compose.Up(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "restarted"})
}

func (s *Server) containerStats(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	stats, err := client.GetContainerStats(context.Background(), record.Name)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, stats)
}

func (s *Server) containerLogs(c *gin.Context) {
	id := c.Param("id")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	tail := c.DefaultQuery("tail", "500")
	reader, err := client.GetContainerLogs(context.Background(), record.Name, tail)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	cleaned := stripDockerHeaders(data)
	ok(c, gin.H{"logs": string(cleaned)})
}

func stripDockerHeaders(data []byte) []byte {
	var result []byte
	i := 0
	for i < len(data) {
		if i+8 > len(data) {
			break
		}
		size := int(data[i+4])<<24 | int(data[i+5])<<16 | int(data[i+6])<<8 | int(data[i+7])
		i += 8
		if i+size > len(data) {
			result = append(result, data[i:]...)
			break
		}
		result = append(result, data[i:i+size]...)
		i += size
	}
	if len(result) == 0 {
		return data
	}
	return result
}

func (s *Server) listFiles(c *gin.Context) {
	id := c.Param("id")
	path := c.DefaultQuery("path", "/")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	entries, err := client.ListContainerFiles(context.Background(), record.Name, path)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, entries)
}

func (s *Server) downloadFile(c *gin.Context) {
	id := c.Param("id")
	path := c.Query("path")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	reader, err := client.DownloadContainerFile(context.Background(), record.Name, path)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.tar", "download"))
	c.Header("Content-Type", "application/x-tar")
	io.Copy(c.Writer, reader)
}

func (s *Server) uploadFile(c *gin.Context) {
	id := c.Param("id")
	dstPath := c.DefaultQuery("path", "/tmp")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	defer file.Close()

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.UploadToContainer(context.Background(), record.Name, dstPath, file); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "uploaded"})
}

func (s *Server) deleteFile(c *gin.Context) {
	id := c.Param("id")
	path := c.Query("path")
	record, err := s.compose.GetContainer(id)
	if err != nil {
		fail(c, 404, "not found")
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.DeleteContainerFile(context.Background(), record.Name, path); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "deleted"})
}

func (s *Server) updateContainerImage(c *gin.Context) {
	id := c.Param("id")

	if err := s.compose.Pull(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	if err := s.compose.Down(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	if err := s.compose.Up(id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	s.compose.SetUpdateAvailable(id, false)
	ok(c, gin.H{"message": "updated"})
}

func (s *Server) checkUpdates(c *gin.Context) {
	s.sched.CheckNow()
	ok(c, gin.H{"message": "update check started"})
}

// ===== IMAGES =====

func (s *Server) listImages(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	images, err := client.ListImages(context.Background())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, images)
}

func (s *Server) pullImage(c *gin.Context) {
	var req struct {
		Image string `json:"image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	reader, err := client.StreamPullImage(context.Background(), req.Image)
	if err != nil {
		c.SSEvent("error", err.Error())
		return
	}
	defer reader.Close()

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			c.SSEvent("progress", string(buf[:n]))
			c.Writer.Flush()
		}
		if err != nil {
			break
		}
	}
	c.SSEvent("done", "pull complete")
}

func (s *Server) loadImage(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	defer file.Close()

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.LoadImage(context.Background(), file); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "image loaded"})
}

func (s *Server) deleteImage(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.RemoveImage(context.Background(), id, force); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "deleted"})
}

// ===== NETWORKS =====

func (s *Server) listNetworks(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	networks, err := client.ListNetworks(context.Background())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, networks)
}

func (s *Server) createNetwork(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if req.Driver == "" {
		req.Driver = "bridge"
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.CreateNetwork(context.Background(), req.Name, req.Driver); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "created"})
}

func (s *Server) deleteNetwork(c *gin.Context) {
	id := c.Param("id")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.RemoveNetwork(context.Background(), id); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "deleted"})
}

func (s *Server) pruneNetworks(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	client.PruneNetworks(context.Background())
	ok(c, gin.H{"message": "pruned"})
}

// ===== VOLUMES =====

func (s *Server) listVolumes(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	volumes, err := client.ListVolumes(context.Background())
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, volumes)
}

func (s *Server) createVolume(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.CreateVolume(context.Background(), req.Name); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "created"})
}

func (s *Server) deleteVolume(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.RemoveVolume(context.Background(), name, force); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "deleted"})
}

func (s *Server) pruneVolumes(c *gin.Context) {
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	client.PruneVolumes(context.Background())
	ok(c, gin.H{"message": "pruned"})
}

// ===== SETTINGS =====

func (s *Server) getSettings(c *gin.Context) {
	settings, err := s.db.GetAllSettings()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	admin, _ := auth.GetAdmin(s.db)
	if admin != nil {
		settings["admin_username"] = admin.Username
	}
	ok(c, settings)
}

func (s *Server) updateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	allowedKeys := map[string]bool{
		"update_check_interval": true,
		"docker_proxy":          true,
	}

	for k, v := range req {
		if !allowedKeys[k] {
			continue
		}
		if err := s.db.SetSetting(k, v); err != nil {
			fail(c, 500, err.Error())
			return
		}
		if k == "update_check_interval" {
			s.sched.UpdateInterval(v)
		}
	}
	ok(c, gin.H{"message": "settings updated"})
}

func (s *Server) updateAdmin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	if err := auth.UpdateAdmin(s.db, req.Username, req.Password); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "admin updated"})
}

func (s *Server) installDocker(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")

	c.SSEvent("info", "Starting Docker installation...")
	c.Writer.Flush()

	c.SSEvent("info", "Run: curl -fsSL https://get.docker.com | sh")
	c.SSEvent("info", "Please run this command manually or configure sudo access")
	c.SSEvent("done", "")
}

func (s *Server) GetContainerLogs(containerName, tail string) (string, error) {
	client, err := docker.NewClient()
	if err != nil {
		return "", err
	}
	defer client.Close()

	reader, err := client.GetContainerLogs(context.Background(), containerName, tail)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(stripDockerHeaders(data)), nil
}

var _ = strings.TrimPrefix
