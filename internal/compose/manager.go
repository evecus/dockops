package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles compose file storage and container lifecycle.
// No database is used for container state — all runtime data comes from Docker directly.
type Manager struct {
	dataPath string
}

func NewManager(dataPath string) *Manager {
	return &Manager{dataPath: dataPath}
}

// composeDir returns the directory for a given container name.
func (m *Manager) composeDir(name string) string {
	return filepath.Join(m.dataPath, "compose", name)
}

// composePath returns the docker-compose.yml path for a container name.
func (m *Manager) composePath(name string) string {
	return filepath.Join(m.composeDir(name), "docker-compose.yml")
}

// HasComposeFile returns true if a compose file exists for the given name.
func (m *Manager) HasComposeFile(name string) bool {
	_, err := os.Stat(m.composePath(name))
	return err == nil
}

// GetComposeDir returns the compose directory path for display.
func (m *Manager) GetComposeDir(name string) string {
	return m.composeDir(name)
}

// WriteCompose writes compose content to disk for the given name.
func (m *Manager) WriteCompose(name, content string) error {
	dir := m.composeDir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(m.composePath(name), []byte(content), 0644)
}

// RemoveComposeDir deletes the compose directory for a container name.
func (m *Manager) RemoveComposeDir(name string) {
	os.RemoveAll(m.composeDir(name))
}

// ContainerExists checks whether a docker container with the given name exists.
func ContainerExists(name string) bool {
	out, err := exec.Command("docker", "ps", "-a", "--filter", "name=^/"+name+"$", "--format", "{{.Names}}").Output()
	if err != nil {
		return false
	}
	for _, n := range strings.Fields(string(out)) {
		if n == name {
			return true
		}
	}
	return false
}

// StopAndRemove forcefully stops and removes a container by its exact name.
func StopAndRemove(name string) {
	exec.Command("docker", "rm", "-f", name).Run()
}

// Up runs docker compose up for the given container name.
func (m *Manager) Up(name string) error {
	p := m.composePath(name)
	cmd := exec.CommandContext(context.Background(), "docker", "compose", "-f", p, "up", "-d", "--pull", "missing")
	cmd.Dir = m.composeDir(name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up failed: %s: %w", string(out), err)
	}
	return nil
}

// Down runs docker compose down for the given container name.
func (m *Manager) Down(name string) error {
	p := m.composePath(name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}
	cmd := exec.CommandContext(context.Background(), "docker", "compose", "-f", p, "down")
	cmd.Dir = m.composeDir(name)
	cmd.Run()
	return nil
}

// Pull runs docker compose pull for the given container name.
func (m *Manager) Pull(name string) error {
	p := m.composePath(name)
	cmd := exec.Command("docker", "compose", "-f", p, "pull")
	cmd.Dir = m.composeDir(name)
	return cmd.Run()
}

// FormFields is the structured data for the edit/create form.
type FormFields struct {
	Image       string   `json:"image"`
	Restart     string   `json:"restart"`
	Hostname    string   `json:"hostname"`
	Privileged  bool     `json:"privileged"`
	Ports       []string `json:"ports"`
	Volumes     []string `json:"volumes"`
	Env         []string `json:"env"`
	Command     string   `json:"command"`
	Entrypoint  string   `json:"entrypoint"`
	User        string   `json:"user"`
	NetworkMode string   `json:"network_mode"`
}

// BuildComposeFromFields generates docker-compose.yml content from FormFields.
func BuildComposeFromFields(name string, f *FormFields) string {
	svcName := name
	if svcName == "" {
		svcName = "app"
	}
	y := fmt.Sprintf("version: '3.8'\n\nservices:\n  %s:\n    image: %s\n    container_name: %s\n", svcName, f.Image, svcName)
	if f.Restart != "" {
		y += fmt.Sprintf("    restart: %s\n", f.Restart)
	}
	if f.Hostname != "" {
		y += fmt.Sprintf("    hostname: %s\n", f.Hostname)
	}
	if f.Privileged {
		y += "    privileged: true\n"
	}
	if f.NetworkMode != "" {
		y += fmt.Sprintf("    network_mode: %s\n", f.NetworkMode)
	}
	if f.Command != "" {
		y += fmt.Sprintf("    command: %s\n", f.Command)
	}
	if f.Entrypoint != "" {
		y += fmt.Sprintf("    entrypoint: %s\n", f.Entrypoint)
	}
	if f.User != "" {
		y += fmt.Sprintf("    user: \"%s\"\n", f.User)
	}
	ports := filterEmpty(f.Ports)
	if len(ports) > 0 {
		y += "    ports:\n"
		for _, p := range ports {
			y += fmt.Sprintf("      - \"%s\"\n", p)
		}
	}
	vols := filterEmpty(f.Volumes)
	if len(vols) > 0 {
		y += "    volumes:\n"
		for _, v := range vols {
			y += fmt.Sprintf("      - %s\n", v)
		}
	}
	envs := filterEmpty(f.Env)
	if len(envs) > 0 {
		y += "    environment:\n"
		for _, e := range envs {
			y += fmt.Sprintf("      - %s\n", e)
		}
	}
	return y
}

func filterEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// ExtractNameFromCompose extracts the container name from compose YAML.
// It prefers container_name if set, otherwise falls back to the first service name.
func ExtractNameFromCompose(content string) string {
	var firstService string
	inServices := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "services:" {
			inServices = true
			continue
		}
		if !inServices {
			continue
		}
		// container_name takes priority
		if strings.HasPrefix(trimmed, "container_name:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "container_name:"))
			val = strings.Trim(val, "\"'")
			if val != "" {
				return val
			}
		}
		// Capture first service name (indented key ending with colon, no sub-indent)
		if firstService == "" && strings.HasSuffix(trimmed, ":") &&
			!strings.Contains(trimmed, " ") && len(line)-len(strings.TrimLeft(line, " \t")) > 0 {
			firstService = strings.TrimSuffix(trimmed, ":")
		}
	}
	return firstService
}

// ExtractImageFromCompose extracts image names from compose YAML content.
func ExtractImageFromCompose(content string) []string {
	var images []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "image:") {
			img := strings.TrimSpace(strings.TrimPrefix(line, "image:"))
			img = strings.Trim(img, "\"'")
			if img != "" {
				images = append(images, img)
			}
		}
	}
	return images
}
