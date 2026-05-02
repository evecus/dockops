package ws

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dockops/dockops/internal/auth"
	"github.com/dockops/dockops/internal/docker"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type TerminalMessage struct {
	Type string `json:"type"` // input, resize
	Data string `json:"data"`
	Rows uint   `json:"rows"`
	Cols uint   `json:"cols"`
}

func HandleTerminal(w http.ResponseWriter, r *http.Request, containerID string) {
	token := r.URL.Query().Get("token")
	if _, err := auth.ValidateToken(token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	dockerClient, err := docker.NewClient()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to connect to Docker: "+err.Error()))
		return
	}
	defer dockerClient.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	execID, err := dockerClient.ContainerExecCreate(ctx, containerID, []string{"/bin/sh", "-c",
		"if command -v bash > /dev/null 2>&1; then bash; else sh; fi"})
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to create exec: "+err.Error()))
		return
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, execID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Failed to attach exec: "+err.Error()))
		return
	}
	defer resp.Close()

	// Send docker output to WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := resp.Reader.Read(buf)
			if err != nil {
				cancel()
				return
			}
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	conn.SetReadDeadline(time.Time{})
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var tmsg TerminalMessage
		if err := json.Unmarshal(msg, &tmsg); err == nil {
			switch tmsg.Type {
			case "input":
				resp.Conn.Write([]byte(tmsg.Data))
			case "resize":
				dockerClient.ResizeTTY(ctx, execID, tmsg.Rows, tmsg.Cols)
			}
		} else {
			resp.Conn.Write(msg)
		}
	}
}

func HandleLogs(w http.ResponseWriter, r *http.Request, containerID string) {
	token := r.URL.Query().Get("token")
	if _, err := auth.ValidateToken(token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	dockerClient, err := docker.NewClient()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer dockerClient.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := dockerClient.StreamLogs(ctx, containerID)
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer reader.Close()

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			data := buf[:n]
			// Strip docker multiplexed stream header (8 bytes)
			if len(data) > 8 {
				data = data[8:]
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
		if err == io.EOF || err != nil {
			return
		}
	}
}
