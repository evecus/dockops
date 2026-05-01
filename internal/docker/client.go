package docker

import (
	"archive/tar"
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
	"github.com/docker/docker/api/types/network"
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
		DockerVersion: info.ServerVersion, OS: info.OperatingSystem, Arch: info.Architecture,
		KernelVersion: info.KernelVersion, TotalMemory: info.MemTotal, CPUs: info.NCPU,
		StorageDriver: info.Driver, LoggingDriver: info.LoggingDriver, DockerRootDir: info.DockerRootDir,
		Containers: info.Containers, ContainersPaused: info.ContainersPaused,
		ContainersStopped: info.ContainersStopped, ContainersRunning: info.ContainersRunning,
		Images: info.Images, ServerTime: time.Now().Format(time.RFC3339),
	}, nil
}

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
				HostIP: p.IP, HostPort: fmt.Sprintf("%d", p.PublicPort),
				ContainerPort: fmt.Sprintf("%d", p.PrivatePort), Protocol: p.Type,
			})
		}
		shortID := ct.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		result = append(result, ContainerSummary{
			ID: shortID, Name: name, Image: ct.Image, ImageID: ct.ImageID,
			Status: ct.Status, State: ct.State, Ports: ports, Created: ct.Created, Labels: ct.Labels,
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
			ports = append(ports, PortBinding{HostIP: b.HostIP, HostPort: b.HostPort, ContainerPort: parts[0], Protocol: proto})
		}
	}
	var mounts []MountInfo
	for _, m := range info.Mounts {
		mounts = append(mounts, MountInfo{Type: string(m.Type), Source: m.Source, Destination: m.Destination, Mode: m.Mode})
	}
	networks := make(map[string]string)
	for k, v := range info.NetworkSettings.Networks {
		networks[k] = v.IPAddress
	}
	shortID := info.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return &ContainerDetail{
		ContainerSummary: ContainerSummary{ID: shortID, Name: name, Image: info.Config.Image, ImageID: info.Image,
			Status: info.State.Status, State: info.State.Status, Ports: ports, Created: info.Created.Unix()},
		Hostname: info.Config.Hostname, Env: info.Config.Env, Mounts: mounts, Networks: networks,
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
	return &ContainerStats{CPUPercent: cpuPct, MemoryUsage: s.MemoryStats.Usage,
		MemoryLimit: s.MemoryStats.Limit, MemoryPercent: memPct, NetRxBytes: netRx, NetTxBytes: netTx,
		BlockRead: blockR, BlockWrite: blockW}, nil
}

func (c *Client) ContainerExecCreate(ctx context.Context, id string, cmd []string) (string, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, id, types.ExecConfig{
		Cmd: cmd, AttachStdin: true, AttachStdout: true, AttachStderr: true, Tty: true,
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
	return c.cli.ContainerExecResize(ctx, execID, types.ResizeOptions{Height: h, Width: w})
}

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

func (c *Client) ListContainerFiles(ctx context.Context, id, path string) ([]FileEntry, error) {
	content, _, err := c.cli.CopyFromContainer(ctx, id, path)
	if err != nil {
		return nil, err
	}
	defer content.Close()
	var entries []FileEntry
	tr := tar.NewReader(content)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Base(hdr.Name)
		if name == "." || name == "" {
			continue
		}
		entries = append(entries, FileEntry{
			Name: name, Path: filepath.Join(path, name), Size: hdr.Size,
			Mode: hdr.FileInfo().Mode().String(), ModTime: hdr.ModTime.Unix(),
			IsDir: hdr.Typeflag == tar.TypeDir,
		})
	}
	return entries, nil
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
		Cmd: []string{"rm", "-rf", path}, AttachStdout: true, AttachStderr: true,
	})
	if err != nil {
		return err
	}
	return c.cli.ContainerExecStart(ctx, exec.ID, types.ExecStartCheck{})
}

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
		result = append(result, ImageSummary{ID: id, RepoTags: img.RepoTags, Size: img.Size, Created: img.Created})
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
	nets, err := c.cli.NetworkList(ctx, network.ListOptions{})
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
			ID: id, Name: n.Name, Driver: n.Driver, Scope: n.Scope,
			IPv4: ipv4, IPv6: ipv6, Internal: n.Internal,
			Created: n.Created.Format(time.RFC3339), Containers: len(n.Containers),
		})
	}
	return result, nil
}

func (c *Client) CreateNetwork(ctx context.Context, name, driver string) error {
	_, err := c.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: driver})
	return err
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	return c.cli.NetworkRemove(ctx, id)
}

func (c *Client) PruneNetworks(ctx context.Context) error {
	_, err := c.cli.NetworksPrune(ctx, filters.Args{})
	return err
}

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
			Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint, Scope: v.Scope, Created: v.CreatedAt,
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
	_, err := c.cli.VolumesPrune(ctx, filters.Args{})
	return err
}

func (c *Client) StreamLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true, Tail: "200", Timestamps: true,
	})
}

func (c *Client) GetClient() *client.Client { return c.cli }
