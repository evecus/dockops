package handler

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

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
	version string
}

func NewServer(cfg *config.Config, database *db.DB, webFS embed.FS, sched *scheduler.Scheduler, version string) *Server {
	return &Server{
		cfg:     cfg,
		db:      database,
		webFS:   webFS,
		sched:   sched,
		compose: compose.NewManager(cfg.DataPath),
		version: version,
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
			auth.POST("/dashboard/refresh", s.dashboardRefresh)

			// Containers — :name is the docker container name
			auth.GET("/containers", s.listContainers)
			auth.POST("/containers", s.createContainer)
			auth.POST("/containers/stream", s.createContainerStream)
			auth.POST("/containers/parse-run", s.parseDockerRun)
			auth.GET("/containers/:name", s.getContainer)
			auth.GET("/containers/:name/form-data", s.getContainerFormData)
			auth.PUT("/containers/:name", s.updateContainer)
			auth.DELETE("/containers/:name", s.deleteContainer)
			auth.POST("/containers/:name/start", s.startContainer)
			auth.POST("/containers/:name/stop", s.stopContainer)
			auth.POST("/containers/:name/restart", s.restartContainer)
			auth.GET("/containers/:name/stats", s.containerStats)
			auth.GET("/containers/:name/logs", s.containerLogs)
			auth.GET("/containers/:name/files", s.listFiles)
			auth.GET("/containers/:name/files/download", s.downloadFile)
			auth.POST("/containers/:name/files/upload", s.uploadFile)
			auth.DELETE("/containers/:name/files", s.deleteFile)
			auth.POST("/containers/:name/update", s.updateContainerImage)

			// Images
			auth.GET("/images", s.listImages)
			auth.GET("/images/check-update", s.checkImageUpdate)
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

	r.GET("/ws/containers/:name/terminal", func(c *gin.Context) {
		ws.HandleTerminal(c.Writer, c.Request, c.Param("name"))
	})
	r.GET("/ws/containers/:name/logs", func(c *gin.Context) {
		ws.HandleLogs(c.Writer, c.Request, c.Param("name"))
	})

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
	ok(c, gin.H{"setup": isSetup, "version": s.version})
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
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	token, err := auth.Login(s.db, req.Username, req.Password)
	if err != nil {
		fail(c, 401, err.Error())
		return
	}
	ok(c, gin.H{"token": token})
}

// ===== DASHBOARD =====

func (s *Server) dashboardInfo(c *gin.Context) {
	info, _, _ := s.sched.Cache.Get()
	if info == nil {
		client, err := docker.NewClient()
		if err != nil {
			fail(c, 500, err.Error())
			return
		}
		defer client.Close()
		info, err = client.GetSystemInfo(context.Background())
		if err != nil {
			fail(c, 500, err.Error())
			return
		}
	}
	ok(c, info)
}

func (s *Server) dashboardStats(c *gin.Context) {
	_, stats, _ := s.sched.Cache.Get()
	if stats == nil {
		ok(c, gin.H{})
		return
	}
	ok(c, stats)
}

func (s *Server) dashboardRefresh(c *gin.Context) {
	// Collect synchronously so the response contains fresh data
	s.sched.CollectNow()
	// Wait briefly for collection to complete, then return fresh data
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	ctx := context.Background()
	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}

	containers, _ := client.ListContainers(ctx)
	var totalCPU float64
	var totalMem, totalMemLimit uint64
	for _, ct := range containers {
		if ct.State == "running" {
			if st, err := client.GetContainerStats(ctx, ct.ID); err == nil {
				totalCPU += st.CPUPercent
				totalMem += st.MemoryUsage
				totalMemLimit += st.MemoryLimit
			}
		}
	}

	ok(c, gin.H{
		"info": info,
		"stats": gin.H{
			"total_cpu_percent": totalCPU,
			"total_mem_usage":   totalMem,
			"total_mem_limit":   totalMemLimit,
			"containers":        len(containers),
		},
	})
}

// ===== CONTAINERS =====

// ContainerInfo is what we return in the list.
type ContainerInfo struct {
	Name        string              `json:"name"`
	Image       string              `json:"image"`
	State       string              `json:"state"`
	Status      string              `json:"status"`
	Ports       []docker.PortBinding `json:"ports"`
	HasCompose  bool                `json:"has_compose"`
	ComposeDir  string              `json:"compose_dir,omitempty"`
}

// listContainers returns all docker containers with real-time data.
func (s *Server) listContainers(c *gin.Context) {
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

	var result []ContainerInfo
	for _, ct := range containers {
		info := ContainerInfo{
			Name:       ct.Name,
			Image:      ct.Image,
			State:      ct.State,
			Status:     ct.Status,
			Ports:      ct.Ports,
			HasCompose: s.compose.HasComposeFile(ct.Name),
		}
		if info.HasCompose {
			info.ComposeDir = s.compose.GetComposeDir(ct.Name)
		}
		result = append(result, info)
	}
	ok(c, result)
}

// createContainerStream handles SSE streaming creation of a container.
// The client sends name + compose_content as query params or POST body via fetch,
// and receives server-sent events with progress lines.
func (s *Server) createContainerStream(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		ComposeContent string `json:"compose_content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.SSEvent("error", err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = compose.ExtractNameFromCompose(req.ComposeContent)
	}
	if name == "" {
		c.SSEvent("error", "cannot determine container name from compose content")
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	send := func(event, data string) {
		c.SSEvent(event, data)
		c.Writer.Flush()
	}

	send("info", fmt.Sprintf("正在写入 Compose 配置: data/compose/%s/", name))
	if err := s.compose.WriteCompose(name, req.ComposeContent); err != nil {
		send("error", "写入配置失败: "+err.Error())
		return
	}

	send("info", "正在拉取镜像并启动容器...")

	lines := make(chan string, 64)
	var upErr error

	go func() {
		defer close(lines)
		upErr = s.compose.UpStream(name, lines)
	}()

	for line := range lines {
		send("log", line)
	}

	if upErr != nil {
		s.compose.RemoveComposeDir(name)
		send("error", "启动失败: "+upErr.Error())
		return
	}

	send("done", name)
}

// createContainer creates a new container.
// For form mode, name is provided explicitly.
// For upload/paste/run modes, name is extracted from the compose content.
func (s *Server) createContainer(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		ComposeContent string `json:"compose_content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}

	// If name not provided, extract from compose content
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = compose.ExtractNameFromCompose(req.ComposeContent)
	}
	if name == "" {
		fail(c, 400, "cannot determine container name from compose content")
		return
	}

	if err := s.compose.WriteCompose(name, req.ComposeContent); err != nil {
		fail(c, 500, err.Error())
		return
	}

	if err := s.compose.Up(name); err != nil {
		// Clean up compose dir if start failed
		s.compose.RemoveComposeDir(name)
		fail(c, 500, "Container created but failed to start: "+err.Error())
		return
	}

	ok(c, gin.H{"message": "created", "name": name})
}

func (s *Server) parseDockerRun(c *gin.Context) {
	var req struct {
		Command string `json:"command"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	result, err := parser.ParseDockerRun(req.Command)
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	ok(c, gin.H{
		"yaml":    result.ToYAML(),
		"service": result,
	})
}

// getContainer returns real-time docker inspect data for a container.
func (s *Server) getContainer(c *gin.Context) {
	name := c.Param("name")
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	detail, err := client.InspectContainer(context.Background(), name)
	if err != nil {
		fail(c, 404, "container not found")
		return
	}
	ok(c, detail)
}

// getContainerFormData returns pre-filled form fields from docker inspect.
func (s *Server) getContainerFormData(c *gin.Context) {
	name := c.Param("name")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	detail, err := client.InspectContainer(context.Background(), name)
	if err != nil {
		fail(c, 404, "container not found: "+err.Error())
		return
	}

	fields := &compose.FormFields{
		Image:      detail.Image,
		Hostname:   detail.Hostname,
		Privileged: false,
		Restart:    detail.RestartPolicy,
	}

	for _, p := range detail.Ports {
		if p.HostPort != "" && p.HostPort != "0" {
			fields.Ports = append(fields.Ports, fmt.Sprintf("%s:%s/%s", p.HostPort, p.ContainerPort, p.Protocol))
		}
	}

	for _, m := range detail.Mounts {
		if m.Type == "bind" || m.Type == "volume" {
			fields.Volumes = append(fields.Volumes, fmt.Sprintf("%s:%s", m.Source, m.Destination))
		}
	}

	for _, e := range detail.Env {
		if strings.Contains(e, "=") {
			fields.Env = append(fields.Env, e)
		}
	}

	for net := range detail.Networks {
		if net == "bridge" || net == "host" || net == "none" {
			fields.NetworkMode = net
		}
		break
	}

	ok(c, gin.H{
		"name":   name,
		"fields": fields,
	})
}

// updateContainer: stop and remove old container, write new compose, start new container.
// oldName is the current docker container name (passed as :name in URL).
// Body contains newName + compose_content.
func (s *Server) updateContainer(c *gin.Context) {
	oldName := c.Param("name")

	var req struct {
		Name           string `json:"name" binding:"required"`
		ComposeContent string `json:"compose_content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err.Error())
		return
	}
	newName := req.Name

	// Validate: newName must not conflict with other existing containers (can equal oldName)
	if newName != oldName && compose.ContainerExists(newName) {
		fail(c, 400, fmt.Sprintf("container '%s' already exists", newName))
		return
	}

	// Step 1: stop and remove the old container (exact name from docker)
	compose.StopAndRemove(oldName)

	// Step 2: if renamed, remove old compose dir
	if newName != oldName {
		s.compose.RemoveComposeDir(oldName)
	}

	// Step 3: write new compose file
	if err := s.compose.WriteCompose(newName, req.ComposeContent); err != nil {
		fail(c, 500, err.Error())
		return
	}

	// Step 4: start new container
	if err := s.compose.Up(newName); err != nil {
		fail(c, 500, "Updated but failed to start: "+err.Error())
		return
	}

	ok(c, gin.H{"message": "updated", "name": newName})
}

// deleteContainer stops, removes the container and deletes its compose dir.
func (s *Server) deleteContainer(c *gin.Context) {
	name := c.Param("name")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	// Stop first (ignore error if already stopped), then force remove
	_ = client.StopContainer(context.Background(), name)
	if err := client.RemoveContainer(context.Background(), name, true); err != nil {
		fail(c, 500, err.Error())
		return
	}

	// Clean up compose dir if present
	s.compose.RemoveComposeDir(name)
	ok(c, gin.H{"message": "deleted"})
}

func (s *Server) startContainer(c *gin.Context) {
	name := c.Param("name")

	if s.compose.HasComposeFile(name) {
		if err := s.compose.Up(name); err != nil {
			fail(c, 500, err.Error())
			return
		}
	} else {
		client, err := docker.NewClient()
		if err != nil {
			fail(c, 500, err.Error())
			return
		}
		defer client.Close()
		if err := client.StartContainer(context.Background(), name); err != nil {
			fail(c, 500, err.Error())
			return
		}
	}
	ok(c, gin.H{"message": "started"})
}

func (s *Server) stopContainer(c *gin.Context) {
	name := c.Param("name")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()
	if err := client.StopContainer(context.Background(), name); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "stopped"})
}

func (s *Server) restartContainer(c *gin.Context) {
	name := c.Param("name")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()
	if err := client.RestartContainer(context.Background(), name); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "restarted"})
}

func (s *Server) containerStats(c *gin.Context) {
	name := c.Param("name")
	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	stats, err := client.GetContainerStats(context.Background(), name)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, stats)
}

func (s *Server) containerLogs(c *gin.Context) {
	name := c.Param("name")
	tail := c.DefaultQuery("tail", "500")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	reader, err := client.GetContainerLogs(context.Background(), name, tail)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer reader.Close()

	data, _ := io.ReadAll(reader)
	ok(c, gin.H{"logs": string(stripDockerHeaders(data))})
}

func (s *Server) listFiles(c *gin.Context) {
	name := c.Param("name")
	path := c.DefaultQuery("path", "/")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	entries, err := client.ListContainerFiles(context.Background(), name, path)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, entries)
}

func (s *Server) downloadFile(c *gin.Context) {
	name := c.Param("name")
	path := c.Query("path")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	reader, err := client.DownloadContainerFile(context.Background(), name, path)
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", "attachment; filename=download.tar")
	c.Header("Content-Type", "application/x-tar")
	io.Copy(c.Writer, reader)
}

func (s *Server) uploadFile(c *gin.Context) {
	name := c.Param("name")
	dstPath := c.DefaultQuery("path", "/tmp")

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

	if err := client.UploadToContainer(context.Background(), name, dstPath, file); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "uploaded"})
}

func (s *Server) deleteFile(c *gin.Context) {
	name := c.Param("name")
	path := c.Query("path")

	client, err := docker.NewClient()
	if err != nil {
		fail(c, 500, err.Error())
		return
	}
	defer client.Close()

	if err := client.DeleteContainerFile(context.Background(), name, path); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "deleted"})
}

func (s *Server) updateContainerImage(c *gin.Context) {
	name := c.Param("name")

	if !s.compose.HasComposeFile(name) {
		fail(c, 400, "no compose file found for this container")
		return
	}

	if err := s.compose.Pull(name); err != nil {
		fail(c, 500, err.Error())
		return
	}
	s.compose.Down(name)
	if err := s.compose.Up(name); err != nil {
		fail(c, 500, err.Error())
		return
	}
	ok(c, gin.H{"message": "updated"})
}

// checkImageUpdate pulls the image and compares IDs to detect if an update is available.
// Streams progress via SSE: "checking" → "pulling" → "up-to-date" or "updated" or "error"
func (s *Server) checkImageUpdate(c *gin.Context) {
	tag := c.Query("tag")
	if tag == "" {
		c.SSEvent("error", "missing tag parameter")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	send := func(event, data string) {
		c.SSEvent(event, data)
		c.Writer.Flush()
	}

	client, err := docker.NewClient()
	if err != nil {
		send("error", "无法连接 Docker: "+err.Error())
		return
	}
	defer client.Close()

	ctx := context.Background()

	// Record current local image ID
	send("checking", "正在获取当前版本信息...")
	oldID, _ := client.GetImageID(ctx, tag)

	// Pull latest
	send("pulling", "正在拉取最新版本...")
	reader, err := client.StreamPullImage(ctx, tag)
	if err != nil {
		send("error", "拉取失败: "+err.Error())
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			send("progress", string(buf[:n]))
		}
		if err != nil {
			break
		}
	}
	reader.Close()

	// Compare IDs
	newID, err := client.GetImageID(ctx, tag)
	if err != nil {
		send("error", "获取新版本信息失败: "+err.Error())
		return
	}

	if oldID != "" && oldID == newID {
		send("up-to-date", "当前版本已是最新版，无需更新")
	} else {
		send("updated", "更新完成！镜像已更新到最新版本")
	}
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
	id, _ := url.QueryUnescape(c.Param("id"))
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
		"docker_proxy":     true,
		"collect_interval": true,
	}
	for k, v := range req {
		if !allowedKeys[k] {
			continue
		}
		if err := s.db.SetSetting(k, v); err != nil {
			fail(c, 500, err.Error())
			return
		}
		if k == "collect_interval" {
			s.sched.UpdateCollectInterval(v)
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
