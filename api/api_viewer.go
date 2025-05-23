package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/PlakarKorp/kloset/appcontext"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/viewers"
)

type ViewerAPI struct {
	ctx  *appcontext.AppContext
	repo *repository.Repository
}

func NewViewerAPI(ctx *appcontext.AppContext, repo *repository.Repository) *ViewerAPI {
	return &ViewerAPI{
		ctx,
		repo,
	}
}

type Viewer struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

var Viewers = []Viewer{
	{Id: "terminal", Name: "Terminal"},
	{Id: "postgres", Name: "PostgreSQL CLI"},
}

// GetAvailableViewers returns a list of all the available
// viewers. For now, this list is hardcoded.
// In the future, this endpoint will accept the parameters ?snapshot and ?path
// to filter the list of viewers available for the given snapshot and
// path.
// Also, in the future, a plugin system will be implemented to allow extending
// the list of available viewers.
func (api *ViewerAPI) GetAvailableViewers(w http.ResponseWriter, r *http.Request) error {
	offset, err := QueryParamToUint32(r, "offset", 0, 0)
	if err != nil {
		return err
	}
	limit, err := QueryParamToUint32(r, "limit", 1, 50)
	if err != nil {
		return err

	}

	if offset > uint32(len(Viewers)) {
		return json.NewEncoder(w).Encode(Items[Viewer]{
			Total: len(Viewers),
			Items: []Viewer{},
		})
	}
	if offset+limit > uint32(len(Viewers)) {
		return json.NewEncoder(w).Encode(Items[Viewer]{
			Total: len(Viewers),
			Items: Viewers[offset:],
		})
	}

	return json.NewEncoder(w).Encode(
		Items[Viewer]{
			Total: len(Viewers),
			Items: Viewers[offset : offset+limit],
		})
}

type StartViewerRequest struct {
	Viewer   string `json:"viewer"` // Viewer ID
	Snapshot string `json:"snapshot"`
	Path     string `json:"path"`
}

type StartViewerResponse struct {
	Id       string   `json:"id"`
	Services []string `json:"services"`
}

func (api *ViewerAPI) StartViewer(w http.ResponseWriter, r *http.Request) error {
	var req StartViewerRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode request body: %w", err)
	}

	if req.Viewer == "" {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "viewer is required",
		}
	}
	found := false
	for _, v := range Viewers {
		if v.Id == req.Viewer {
			found = true
			break
		}
	}
	if !found {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "invalid viewer type",
		}
	}

	if req.Snapshot == "" {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "snapshot is required",
		}
	}
	if req.Path == "" {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "path is required",
		}
	}

	var viewer viewers.Viewer

	switch req.Viewer {
	case "terminal":
		viewer = viewers.NewTerminal()
	}

	if viewer == nil {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "invalid viewer type",
		}
	}

	runner, err := viewers.NewRunner(api.repo, viewer)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	status, err := runner.Run(api.ctx, req.Snapshot, req.Path)
	if err != nil {
		return err
	}

	services := []string{}
	for _, service := range status.Services {
		services = append(services, fmt.Sprintf("http://%s", service))
	}

	return json.NewEncoder(w).Encode(StartViewerResponse{
		Id:       runner.Path,
		Services: services,
	})
}

type WebSocketRequest struct {
	Snapshot string `json:"snapshot"`
	Path     string `json:"path"`
	// XXX: add token
}

func (api *ViewerAPI) WebSocket(w http.ResponseWriter, r *http.Request) error {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade connection to WebSocket: %w", err)
	}
	defer ws.Close()

	msgType, message, err := ws.ReadMessage()
	if msgType != websocket.TextMessage {
		return fmt.Errorf("expected text message, got %v", msgType)
	}

	request := WebSocketRequest{}
	if err := json.Unmarshal(message, &request); err != nil {
		return fmt.Errorf("failed to decode request body: %w", err)
	}

	if request.Snapshot == "" {
		// Return an error to the client
		return fmt.Errorf("snapshot is required")
	}
	if request.Path == "" {
		// Return an error to the client
		return fmt.Errorf("path is required")
	}

	cmd := exec.Command("docker", "run", "--rm", "--privileged", "-ti", "test", "-host", "http://host.docker.internal:9888", "-snapshot", request.Snapshot, "-path", request.Path)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}
	defer ptmx.Close()

	go func() {
		for {
			buf := make([]byte, 1024)
			n, err := ptmx.Read(buf)
			if err != nil {
				fmt.Printf("Unable to read from pty: %v\n", err)
				return
			}

			if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				log.Println("Error writing to WebSocket:", err)
				return
			}
		}
	}()

	go func() {
		for {
			_, message, err := ws.ReadMessage()
			fmt.Printf("Received message: %s\n", message)

			if err != nil {
				log.Println("Error reading from WebSocket:", err)
				break
			}

			n, err := ptmx.Write(message)
			if err != nil {
				fmt.Printf("Unable to write to pty: %v\n", err)
				return
			}

			fmt.Printf("written: %v\n", n)
		}
	}()

	cmd.Wait()

	return nil
}
