package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dockops/dockops/internal/db"
	"github.com/google/uuid"
)

type ContainerRecord struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ComposeDir      string `json:"compose_dir"`
	CreateMode      string `json:"create_mode"`
	ComposeContent  string `json:"compose_content"`
	DockerID        string `json:"docker_id"`
	UpdateAvailable bool   `json:"update_available"`
	Source          string `json:"source"` // "dockops" | "external"
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Manager struct {
	db       *db.DB
	dataPath string
}

func NewManager(database *db.DB, dataPath string) *Manager {
	return &Manager{db: database, dataPath: dataPath}
}

func (m *Manager) autoComposeDir(name string) string {
	return filepath.Join(m.dataPath, "compose", name)
}

func (m *Manager) ListContainers() ([]ContainerRecord, error) {
	rows, err := m.db.Query(`
		SELECT id, name, compose_dir, create_mode, compose_content,
		       COALESCE(docker_id,''), update_available, COALESCE(source,'dockops'), created_at, updated_at
		FROM containers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ContainerRecord
	for rows.Next() {
		var r ContainerRecord
		var updateAvail int
		if err := rows.Scan(&r.ID, &r.Name, &r.ComposeDir, &r.CreateMode, &r.ComposeContent,
			&r.DockerID, &updateAvail, &r.Source, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.UpdateAvailable = updateAvail == 1
		result = append(result, r)
	}
	return result, nil
}

func (m *Manager) GetContainer(id string) (*ContainerRecord, error) {
	var r ContainerRecord
	var updateAvail int
	err := m.db.QueryRow(`
		SELECT id, name, compose_dir, create_mode, compose_content,
		       COALESCE(docker_id,''), update_available, COALESCE(source,'dockops'), created_at, updated_at
		FROM containers WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.ComposeDir, &r.CreateMode, &r.ComposeContent,
			&r.DockerID, &updateAvail, &r.Source, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.UpdateAvailable = updateAvail == 1
	return &r, nil
}

func (m *Manager) GetContainerByName(name string) (*ContainerRecord, error) {
	var r ContainerRecord
	var updateAvail int
	err := m.db.QueryRow(`
		SELECT id, name, compose_dir, create_mode, compose_content,
		       COALESCE(docker_id,''), update_available, COALESCE(source,'dockops'), created_at, updated_at
		FROM containers WHERE name = ?`, name).
		Scan(&r.ID, &r.Name, &r.ComposeDir, &r.CreateMode, &r.ComposeContent,
			&r.DockerID, &updateAvail, &r.Source, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.UpdateAvailable = updateAvail == 1
	return &r, nil
}

type CreateRequest struct {
	Name           string `json:"name"`
	CreateMode     string `json:"create_mode"` // upload|paste|run|form
	ComposeContent string `json:"compose_content"`
}

func (m *Manager) CreateContainer(req *CreateRequest) (*ContainerRecord, error) {
	composeDir := m.autoComposeDir(req.Name)

	if err := os.MkdirAll(composeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	composePath := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(req.ComposeContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write compose file: %w", err)
	}

	id := uuid.New().String()
	_, err := m.db.Exec(`
		INSERT INTO containers (id, name, compose_dir, create_mode, compose_content, source)
		VALUES (?, ?, ?, ?, ?, 'dockops')`,
		id, req.Name, composeDir, req.CreateMode, req.ComposeContent)
	if err != nil {
		return nil, err
	}

	return m.GetContainer(id)
}

func (m *Manager) UpdateContainer(id string, req *CreateRequest) error {
	record, err := m.GetContainer(id)
	if err != nil {
		return err
	}

	// Stop existing stack (best-effort; external containers may have no compose file)
	m.composeDown(record.ComposeDir)

	targetName := req.Name
	if targetName == "" {
		targetName = record.Name
	}

	// If the name hasn't changed, reuse the existing compose_dir to avoid path
	// mismatch when data_path has been reconfigured since the record was created.
	// Only compute a new directory when the container is being renamed.
	var composeDir string
	if targetName == record.Name && record.ComposeDir != "" {
		composeDir = record.ComposeDir
	} else {
		composeDir = m.autoComposeDir(targetName)
	}

	if err := os.MkdirAll(composeDir, 0755); err != nil {
		return err
	}

	composePath := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(req.ComposeContent), 0644); err != nil {
		return err
	}

	_, err = m.db.Exec(`
		UPDATE containers SET name=?, compose_dir=?, create_mode=?, compose_content=?,
		                      source='dockops', updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		targetName, composeDir, req.CreateMode, req.ComposeContent, id)
	return err
}

// RegisterExternal upserts a record for a Docker container not created by dockops.
func (m *Manager) RegisterExternal(name string) (*ContainerRecord, error) {
	existing, err := m.GetContainerByName(name)
	if err == nil {
		return existing, nil
	}

	id := uuid.New().String()
	composeDir := m.autoComposeDir(name)
	_, err = m.db.Exec(`
		INSERT INTO containers (id, name, compose_dir, create_mode, compose_content, source)
		VALUES (?, ?, ?, 'external', '', 'external')`,
		id, name, composeDir)
	if err != nil {
		return nil, err
	}
	return m.GetContainer(id)
}

func (m *Manager) DeleteContainer(id string) error {
	record, err := m.GetContainer(id)
	if err != nil {
		return err
	}

	m.composeDown(record.ComposeDir)

	_, err = m.db.Exec(`DELETE FROM containers WHERE id = ?`, id)
	return err
}

func (m *Manager) Up(id string) error {
	record, err := m.GetContainer(id)
	if err != nil {
		return err
	}
	return m.composeUp(record.ComposeDir)
}

func (m *Manager) Down(id string) error {
	record, err := m.GetContainer(id)
	if err != nil {
		return err
	}
	return m.composeDown(record.ComposeDir)
}

func (m *Manager) Pull(id string) error {
	record, err := m.GetContainer(id)
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", "compose", "-f",
		filepath.Join(record.ComposeDir, "docker-compose.yml"), "pull")
	cmd.Dir = record.ComposeDir
	return cmd.Run()
}

func (m *Manager) GetComposeContent(id string) (string, error) {
	record, err := m.GetContainer(id)
	if err != nil {
		return "", err
	}
	return record.ComposeContent, nil
}

func (m *Manager) SetUpdateAvailable(id string, available bool) error {
	val := 0
	if available {
		val = 1
	}
	_, err := m.db.Exec(`UPDATE containers SET update_available = ? WHERE id = ?`, val, id)
	return err
}

func (m *Manager) GetAllForUpdateCheck() ([]ContainerRecord, error) {
	return m.ListContainers()
}

func (m *Manager) composeUp(dir string) error {
	composePath := filepath.Join(dir, "docker-compose.yml")

	// Safety: ensure the directory and compose file exist before invoking docker.
	// This can happen when data_path was reconfigured after the record was created.
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return fmt.Errorf("compose up failed: cannot create directory %s: %w", dir, mkErr)
		}
		// Re-fetch the record whose compose_dir matches dir so we can restore the file.
		rows, qErr := m.db.Query(
			`SELECT compose_content FROM containers WHERE compose_dir = ?`, dir)
		if qErr == nil {
			defer rows.Close()
			if rows.Next() {
				var content string
				if sErr := rows.Scan(&content); sErr == nil && content != "" {
					_ = os.WriteFile(composePath, []byte(content), 0644)
				}
			}
		}
	}

	cmd := exec.CommandContext(context.Background(), "docker", "compose", "-f", composePath, "up", "-d", "--pull", "missing")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up failed: %s: %w", string(out), err)
	}
	return nil
}

func (m *Manager) composeDown(dir string) error {
	composePath := filepath.Join(dir, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return nil
	}
	cmd := exec.CommandContext(context.Background(), "docker", "compose", "-f", composePath, "down")
	cmd.Dir = dir
	cmd.Run()
	return nil
}

// FormFields is the structured data for the edit form.
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
	y := fmt.Sprintf("version: '3.8'\n\nservices:\n  %s:\n    image: %s\n", svcName, f.Image)
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

// ExtractImageFromCompose extracts image name from compose YAML content
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
