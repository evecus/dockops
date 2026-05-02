package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	dockernetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() { c.cli.Close() }

// ===== SYSTEM =====

type SystemInfo struct {
	DockerVersion     string `json:"docker_version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	KernelVersion     string `json:"kernel_version"`
	TotalMemory       int64  `json:"total_memory"`
	CPUs              int    `json:"cpus"`
	StorageDriver     string `json:"storage_driver"`
	LoggingDriver     string `json:"logging_driver"`
	DockerRootDir     string `json:"docker_root_dir"`
	Containers        int    `json:"containers"`
	ContainersPaused  int    `json:"containers_paused"`
	ContainersStopped int    `json:"containers_stopped"`
	ContainersRunning int    `json:"containers_running"`
	Images            int    `json:"images"`
	ServerTime        string `json:"server_time"`
}

func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &SystemInfo{
		DockerVersion:     info.ServerVersion,
		OS:                info.OperatingSystem,
		Arch:              info.Architecture,
		KernelVersion:     info.KernelVersion,
		TotalMemory:       info.MemTotal,
		CPUs:              info.NCPU,
		StorageDriver:     info.Driver,
		LoggingDriver:     info.LoggingDriver,
		DockerRootDir:     info.DockerRootDir,
		Containers:        info.Containers,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		ContainersRunning: info.ContainersRunning,
		Images:            info.Images,
		ServerTime:        time.Now().Format(time.RFC3339),
	}, nil
}

// ===== CONTAINERS =====

type ContainerSummary struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	ImageID string            `json:"image_id"`
	Status  string            `json:"status"`
	State   string            `json:"state"`
	Ports   []PortBinding     `json:"ports"`
	Created int64             `json:"created"`
	Labels  map[string]string `json:"labels"`
}

type PortBinding struct {
	HostIP        string `json:"host_ip"`
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	cts, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	var result []ContainerSummary
	for _, ct := range cts {
		name := strings.TrimPrefix(ct.Names[0], "/")
		var ports []PortBinding
		for _, p := range ct.Ports {
			ports = append(ports, PortBinding{
				HostIP:        p.IP,
				HostPort:      fmt.Sprintf("%d", p.PublicPort),
				ContainerPort: fmt.Sprintf("%d", p.PrivatePort),
				Protocol:      p.Type,
			})
		}
		shortID := ct.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		result = append(result, ContainerSummary{
			ID:      shortID,
			Name:    name,
			Image:   ct.Image,
			ImageID: ct.ImageID,
			Status:  ct.Status,
			State:   ct.State,
			Ports:   ports,
			Created: ct.Created,
			Labels:  ct.Labels,
		})
	}
	return result, nil
}

type ContainerDetail struct {
	ContainerSummary
	Hostname      string            `json:"hostname"`
	Env           []string          `json:"env"`
	Mounts        []MountInfo       `json:"mounts"`
	Networks      map[string]string `json:"networks"`
	RestartPolicy string            `json:"restart_policy"`
}

type MountInfo struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*ContainerDetail, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimPrefix(info.Name, "/")
	var ports []PortBinding
	for port, bindings := range info.HostConfig.PortBindings {
		for _, b := range bindings {
			parts := strings.SplitN(string(port), "/", 2)
			proto := "tcp"
			if len(parts) == 2 {
				proto = parts[1]
			}
			ports = append(ports, PortBinding{
				HostIP:        b.HostIP,
				HostPort:      b.HostPort,
				ContainerPort: parts[0],
				Protocol:      proto,
			})
		}
	}
	var mounts []MountInfo
	for _, m := range info.Mounts {
		mounts = append(mounts, MountInfo{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
		})
	}
	networks := make(map[string]string)
	for k, v := range info.NetworkSettings.Networks {
		networks[k] = v.IPAddress
	}
	shortID := info.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	// info.Created is a string like "2024-01-01T00:00:00Z"
	createdTs := int64(0)
	if t, err := time.Parse(time.RFC3339, info.Created); err == nil {
		createdTs = t.Unix()
	} else if t, err := time.Parse(time.RFC3339Nano, info.Created); err == nil {
		createdTs = t.Unix()
	}

	return &ContainerDetail{
		ContainerSummary: ContainerSummary{
			ID:      shortID,
			Name:    name,
			Image:   info.Config.Image,
			ImageID: info.Image,
			Status:  info.State.Status,
			State:   info.State.Status,
			Ports:   ports,
			Created: createdTs,
		},
		Hostname:      info.Config.Hostname,
		Env:           info.Config.Env,
		Mounts:        mounts,
		Networks:      networks,
		RestartPolicy: string(info.HostConfig.RestartPolicy.Name),
	}, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	t := 30
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	t := 30
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &t})
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force})
}

// ===== STATS =====

type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetRxBytes    uint64  `json:"net_rx_bytes"`
	NetTxBytes    uint64  `json:"net_tx_bytes"`
	BlockRead     uint64  `json:"block_read"`
	BlockWrite    uint64  `json:"block_write"`
}

func (c *Client) GetContainerStats(ctx context.Context, id string) (*ContainerStats, error) {
	resp, err := c.cli.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var s types.StatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}

	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	numCPU := float64(s.CPUStats.OnlineCPUs)
	if numCPU == 0 {
		numCPU = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	cpuPct := 0.0
	if sysDelta > 0 {
		cpuPct = (cpuDelta / sysDelta) * numCPU * 100.0
	}
	memPct := 0.0
	if s.MemoryStats.Limit > 0 {
		memPct = float64(s.MemoryStats.Usage) / float64(s.MemoryStats.Limit) * 100.0
	}
	var netRx, netTx uint64
	for _, v := range s.Networks {
		netRx += v.RxBytes
		netTx += v.TxBytes
	}
	var blockR, blockW uint64
	for _, v := range s.BlkioStats.IoServiceBytesRecursive {
		if v.Op == "read" {
			blockR += v.Value
		} else if v.Op == "write" {
			blockW += v.Value
		}
	}
	return &ContainerStats{
		CPUPercent:    cpuPct,
		MemoryUsage:   s.MemoryStats.Usage,
		MemoryLimit:   s.MemoryStats.Limit,
		MemoryPercent: memPct,
		NetRxBytes:    netRx,
		NetTxBytes:    netTx,
		BlockRead:     blockR,
		BlockWrite:    blockW,
	}, nil
}

// ===== EXEC / TERMINAL =====

func (c *Client) ContainerExecCreate(ctx context.Context, id string, cmd []string) (string, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, id, types.ExecConfig{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) ContainerExecAttach(ctx context.Context, execID string) (types.HijackedResponse, error) {
	return c.cli.ContainerExecAttach(ctx, execID, types.ExecStartCheck{Tty: true})
}

func (c *Client) ResizeTTY(ctx context.Context, execID string, h, w uint) error {
	// Use container resize - compatible across Docker SDK versions
	return c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: h,
		Width:  w,
	})
}

// ===== FILE BROWSER =====

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

func (c *Client) ListContainerFiles(ctx context.Context, id, path string) ([]FileEntry, error) {
	// Use "ls -la --time-style=+%s" via exec to reliably list directory contents.
	// CopyFromContainer on "/" tries to stream the whole FS and returns nothing useful.
	execID, err := c.cli.ContainerExecCreate(ctx, id, types.ExecConfig{
		Cmd:          []string{"ls", "-la", "--time-style=+%s", path},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		// Fallback: try busybox-style ls (no --time-style)
		execID, err = c.cli.ContainerExecCreate(ctx, id, types.ExecConfig{
			Cmd:          []string{"ls", "-la", path},
			AttachStdout: true,
			AttachStderr: true,
		})
		if err != nil {
			return nil, err
		}
	}

	resp, err := c.cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	var buf strings.Builder
	// Docker multiplexed stream: 8-byte header per frame
	header := make([]byte, 8)
	for {
		_, err := io.ReadFull(resp.Reader, header)
		if err != nil {
			break
		}
		size := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		if size == 0 {
			continue
		}
		payload := make([]byte, size)
		_, err = io.ReadFull(resp.Reader, payload)
		if err != nil {
			break
		}
		buf.Write(payload)
	}

	output := buf.String()
	var entries []FileEntry

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Skip "total N", empty lines, and "." ".." entries
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		entry := parseLsLine(line, path)
		if entry == nil {
			continue
		}
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		entries = append(entries, *entry)
	}
	return entries, nil
}

// parseLsLine parses a line from "ls -la" output.
// Handles both GNU ls (with --time-style=+%s giving unix ts) and busybox ls.
func parseLsLine(line, dirPath string) *FileEntry {
	// Fields: permissions links owner group size [date/time...] name
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return nil
	}

	perms := fields[0]
	isDir := len(perms) > 0 && perms[0] == 'd'
	isLink := len(perms) > 0 && perms[0] == 'l'
	_ = isLink

	// Try to find size and name
	// GNU ls -la --time-style=+%s: perms links owner group size timestamp name
	// That's fields[0..6], name at fields[6]
	// busybox ls -la: perms links owner group size month day time name
	// That's fields[0..8], name at fields[8]

	var size int64
	var modTime int64
	var name string

	if len(fields) >= 7 {
		// Try GNU format: fields[4]=size, fields[5]=unix_timestamp, fields[6]=name
		fmt.Sscanf(fields[4], "%d", &size)
		ts, err := parseTimestamp(fields[5])
		if err == nil {
			modTime = ts
			// Name: rejoin from fields[6] onward (handles spaces), strip symlink " -> target"
			name = strings.Join(fields[6:], " ")
		} else if len(fields) >= 9 {
			// Busybox: fields[4]=size, fields[5-7]=date, fields[8]=name
			name = strings.Join(fields[8:], " ")
			// Try to parse date: "Jan  2 15:04" or "Jan  2  2024"
			dateStr := strings.Join(fields[5:8], " ")
			if t, err2 := time.Parse("Jan  2 2006", dateStr); err2 == nil {
				modTime = t.Unix()
			} else if t, err2 := time.Parse("Jan 2 2006", dateStr); err2 == nil {
				modTime = t.Unix()
			}
		} else {
			name = strings.Join(fields[6:], " ")
		}
	}

	if name == "" {
		return nil
	}

	// Strip symlink arrow
	if idx := strings.Index(name, " -> "); idx != -1 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return nil
	}

	entryPath := filepath.Join(dirPath, name)

	return &FileEntry{
		Name:    name,
		Path:    entryPath,
		Size:    size,
		Mode:    perms,
		ModTime: modTime,
		IsDir:   isDir,
	}
}

func parseTimestamp(s string) (int64, error) {
	var ts int64
	_, err := fmt.Sscanf(s, "%d", &ts)
	if err != nil || ts <= 0 {
		return 0, fmt.Errorf("not a unix timestamp")
	}
	return ts, nil
}

func (c *Client) DownloadContainerFile(ctx context.Context, id, path string) (io.ReadCloser, error) {
	content, _, err := c.cli.CopyFromContainer(ctx, id, path)
	return content, err
}

func (c *Client) UploadToContainer(ctx context.Context, id, dstPath string, content io.Reader) error {
	return c.cli.CopyToContainer(ctx, id, dstPath, content, types.CopyToContainerOptions{})
}

func (c *Client) DeleteContainerFile(ctx context.Context, id, path string) error {
	exec, err := c.cli.ContainerExecCreate(ctx, id, types.ExecConfig{
		Cmd:          []string{"rm", "-rf", path},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return err
	}
	return c.cli.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
}

// ===== IMAGES =====

type ImageSummary struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
}

func (c *Client) ListImages(ctx context.Context) ([]ImageSummary, error) {
	imgs, err := c.cli.ImageList(ctx, image.ListOptions{All: false})
	if err != nil {
		return nil, err
	}
	var result []ImageSummary
	for _, img := range imgs {
		id := img.ID
		if strings.HasPrefix(id, "sha256:") && len(id) > 19 {
			id = id[7:19]
		}
		result = append(result, ImageSummary{
			ID:       id,
			RepoTags: img.RepoTags,
			Size:     img.Size,
			Created:  img.Created,
		})
	}
	return result, nil
}

func (c *Client) RemoveImage(ctx context.Context, id string, force bool) error {
	_, err := c.cli.ImageRemove(ctx, id, image.RemoveOptions{Force: force})
	return err
}

func (c *Client) LoadImage(ctx context.Context, r io.Reader) error {
	resp, err := c.cli.ImageLoad(ctx, r, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) StreamPullImage(ctx context.Context, ref string) (io.ReadCloser, error) {
	return c.cli.ImagePull(ctx, ref, image.PullOptions{})
}

func (c *Client) GetImageID(ctx context.Context, ref string) (string, error) {
	inspect, _, err := c.cli.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return "", err
	}
	return inspect.ID, nil
}

// ===== NETWORKS =====

type NetworkSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Scope      string `json:"scope"`
	IPv4       string `json:"ipv4"`
	IPv6       string `json:"ipv6"`
	Internal   bool   `json:"internal"`
	Created    string `json:"created"`
	Containers int    `json:"containers"`
}

func (c *Client) ListNetworks(ctx context.Context) ([]NetworkSummary, error) {
	// Use filters.Args for listing - compatible across versions
	nets, err := c.cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(),
	})
	if err != nil {
		return nil, err
	}
	var result []NetworkSummary
	for _, n := range nets {
		var ipv4, ipv6 string
		for _, cfg := range n.IPAM.Config {
			if strings.Contains(cfg.Subnet, ":") {
				ipv6 = cfg.Subnet
			} else {
				ipv4 = cfg.Subnet
			}
		}
		id := n.ID
		if len(id) > 12 {
			id = id[:12]
		}
		result = append(result, NetworkSummary{
			ID:         id,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			IPv4:       ipv4,
			IPv6:       ipv6,
			Internal:   n.Internal,
			Created:    n.Created.Format(time.RFC3339),
			Containers: len(n.Containers),
		})
	}
	return result, nil
}

func (c *Client) CreateNetwork(ctx context.Context, name, driver string) error {
	_, err := c.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver: driver,
	})
	return err
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	return c.cli.NetworkRemove(ctx, id)
}

func (c *Client) PruneNetworks(ctx context.Context) error {
	_, err := c.cli.NetworksPrune(ctx, filters.NewArgs())
	return err
}

// ===== VOLUMES =====

type VolumeSummary struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Mountpoint string `json:"mountpoint"`
	Scope      string `json:"scope"`
	Created    string `json:"created"`
}

func (c *Client) ListVolumes(ctx context.Context) ([]VolumeSummary, error) {
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}
	var result []VolumeSummary
	for _, v := range resp.Volumes {
		result = append(result, VolumeSummary{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Scope:      v.Scope,
			Created:    v.CreatedAt,
		})
	}
	return result, nil
}

func (c *Client) CreateVolume(ctx context.Context, name string) error {
	_, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	return err
}

func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	return c.cli.VolumeRemove(ctx, name, force)
}

func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.cli.VolumesPrune(ctx, filters.NewArgs())
	return err
}

// ===== LOGS =====

func (c *Client) GetContainerLogs(ctx context.Context, id string, tail string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       tail,
		Timestamps: true,
	})
}

func (c *Client) StreamLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "200",
		Timestamps: true,
	})
}

func (c *Client) GetClient() *client.Client { return c.cli }

// suppress unused import warning
var _ = dockernetwork.EndpointSettings{}
