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
	ID             string `json:"id"`
	Name           string `json:"name"`
	ComposeDir     string `json:"compose_dir"`
	CreateMode     string `json:"create_mode"`
	ComposeContent string `json:"compose_content"`
	DockerID       string `json:"docker_id"`
	UpdateAvailable bool  `json:"update_available"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type Manager struct {
	db *db.DB
}

func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

func (m *Manager) ListContainers() ([]ContainerRecord, error) {
	rows, err := m.db.Query(`
		SELECT id, name, compose_dir, create_mode, compose_content, 
		       COALESCE(docker_id,''), update_available, created_at, updated_at 
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
			&r.DockerID, &updateAvail, &r.CreatedAt, &r.UpdatedAt); err != nil {
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
		       COALESCE(docker_id,''), update_available, created_at, updated_at
		FROM containers WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.ComposeDir, &r.CreateMode, &r.ComposeContent,
			&r.DockerID, &updateAvail, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.UpdateAvailable = updateAvail == 1
	return &r, nil
}

type CreateRequest struct {
	Name           string `json:"name"`
	ComposeDir     string `json:"compose_dir"`
	CreateMode     string `json:"create_mode"` // upload|paste|run|form
	ComposeContent string `json:"compose_content"`
}

func (m *Manager) CreateContainer(req *CreateRequest) (*ContainerRecord, error) {
	// Ensure directory exists
	if err := os.MkdirAll(req.ComposeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	composePath := filepath.Join(req.ComposeDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(req.ComposeContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write compose file: %w", err)
	}

	id := uuid.New().String()
	_, err := m.db.Exec(`
		INSERT INTO containers (id, name, compose_dir, create_mode, compose_content)
		VALUES (?, ?, ?, ?, ?)`,
		id, req.Name, req.ComposeDir, req.CreateMode, req.ComposeContent)
	if err != nil {
		return nil, err
	}

	return m.GetContainer(id)
}

func (m *Manager) UpdateContainer(id string, req *CreateRequest) error {
	// Get existing record
	record, err := m.GetContainer(id)
	if err != nil {
		return err
	}

	// Stop and remove existing compose stack
	m.composeDown(record.ComposeDir)

	// Ensure new dir exists
	if err := os.MkdirAll(req.ComposeDir, 0755); err != nil {
		return err
	}

	composePath := filepath.Join(req.ComposeDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(req.ComposeContent), 0644); err != nil {
		return err
	}

	_, err = m.db.Exec(`
		UPDATE containers SET name=?, compose_dir=?, create_mode=?, compose_content=?, 
		                      updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		req.Name, req.ComposeDir, req.CreateMode, req.ComposeContent, id)
	return err
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
	cmd.Run() // best effort
	return nil
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
