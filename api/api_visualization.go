package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"

	"github.com/PlakarKorp/plakar/pvr"
	"github.com/PlakarKorp/plakar/viewers"
)

type VisualizationAPI struct {
	pvr *pvr.PVR
}

func NewVisualizationAPI(pvr *pvr.PVR) *VisualizationAPI {
	return &VisualizationAPI{
		pvr: pvr,
	}
}

type Visualization struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

var Visualizations = []Visualization{
	{Id: "terminal", Name: "Terminal"},
	{Id: "postgres", Name: "PostgreSQL CLI"},
}

// GetAvailableVisualizations returns a list of all the available
// visualizations. For now, this list is hardcoded.
// In the future, this endpoint will accept the parameters ?snapshot and ?path
// to filter the list of visualizations available for the given snapshot and
// path.
// Also, in the future, a plugin system will be implemented to allow extending
// the list of available visualizations.
func (api *VisualizationAPI) GetAvailableVisualizations(w http.ResponseWriter, r *http.Request) error {
	offset, err := QueryParamToUint32(r, "offset", 0, 0)
	if err != nil {
		return err
	}
	limit, err := QueryParamToUint32(r, "limit", 1, 50)
	if err != nil {
		return err

	}

	if offset > uint32(len(Visualizations)) {
		return json.NewEncoder(w).Encode(Items[Visualization]{
			Total: len(Visualizations),
			Items: []Visualization{},
		})
	}
	if offset+limit > uint32(len(Visualizations)) {
		return json.NewEncoder(w).Encode(Items[Visualization]{
			Total: len(Visualizations),
			Items: Visualizations[offset:],
		})
	}

	return json.NewEncoder(w).Encode(
		Items[Visualization]{
			Total: len(Visualizations),
			Items: Visualizations[offset : offset+limit],
		})
}

func (api *VisualizationAPI) ListRunningVisualizations(w http.ResponseWriter, r *http.Request) error {
	return nil
}

type StartVisualizationRequest struct {
	Visualization string `json:"visualization"` // Visualization ID
	Snapshot      string `json:"snapshot"`
	Path          string `json:"path"`
}

type StartVisualizationResponse struct {
	Id string `json:"id"`
}

func (api *VisualizationAPI) StartVisualization(w http.ResponseWriter, r *http.Request) error {
	var req StartVisualizationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return fmt.Errorf("failed to decode request body: %w", err)
	}

	if req.Visualization == "" {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "visualization is required",
		}
	}
	found := false
	for _, v := range Visualizations {
		if v.Id == req.Visualization {
			found = true
			break
		}
	}
	if !found {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "invalid visualization type",
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

	switch req.Visualization {
	case "terminal":
		viewer = viewers.NewTerminal()
	}

	if viewer == nil {
		return &ApiError{
			HttpCode: 400,
			ErrCode:  "bad-request",
			Message:  "invalid visualization type",
		}
	}

	runner, err := viewers.NewRunner(viewer)
	if err != nil {
		return fmt.Errorf("failed to create runner: %w", err)
	}

	return json.NewEncoder(w).Encode(StartVisualizationResponse{
		Id: runner.Path,
	})
}

type WebSocketRequest struct {
	Snapshot string `json:"snapshot"`
	Path     string `json:"path"`
	// XXX: add token
}

func (api *VisualizationAPI) WebSocket(w http.ResponseWriter, r *http.Request) error {
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
