package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/PlakarKorp/kloset/appcontext"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/viewers"
	"github.com/gorilla/websocket"
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

// XXX: move the list in viewers.go?
var Viewers = []Viewer{
	{Id: "terminal", Name: "Terminal"},
	{Id: "nginx", Name: "Nginx"},
	{Id: "postgres-cli", Name: "PostgreSQL"},
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
	Id         string   `json:"id"`
	Services   []string `json:"services"`
	Attachable bool     `json:"attachable"`
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

	viewer := viewers.NewViewer(req.Viewer)

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

	viewers.Manager.RegisterRunner(runner)

	status, err := runner.Run(api.ctx, req.Snapshot, req.Path)
	if err != nil {
		return err
	}

	services := []string{}
	for _, service := range status.Services {
		services = append(services, fmt.Sprintf("http://%s", service))
	}

	return json.NewEncoder(w).Encode(StartViewerResponse{
		Id:         runner.Path,
		Services:   services,
		Attachable: status.Attachable,
	})
}

type WebSocketRequest struct {
	Viewer string `json:"viewer"` // Viewer ID, returned by StartViewer
	// XXX: add token to authenticate the user
}

func (api *ViewerAPI) WebSocket(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("in websocket\n")
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		err := fmt.Errorf("failed to upgrade connection to WebSocket: %w", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	defer ws.Close()

	msgType, message, err := ws.ReadMessage()
	if err != nil {
		err := fmt.Errorf("failed to read message: %w", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	if msgType != websocket.TextMessage {
		err := fmt.Errorf("expected text message, got %v", msgType)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	request := WebSocketRequest{}
	if err := json.Unmarshal(message, &request); err != nil {
		err := fmt.Errorf("failed to decode request body: %w", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	if request.Viewer == "" {
		err := fmt.Errorf("viewer is required")
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	runner, ok := viewers.Manager.GetRunner(request.Viewer)
	if !ok {
		fmt.Fprintf(os.Stderr, "getRunner error: %v\n", err)
		return
	}

	cmd, ptmx, err := runner.Attach()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attach error: %v", err)
	}

	defer ptmx.Close()

	// XXX: question: is it possible to use a Pipe instead of copy to/from the websocket manually?
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

			if err != nil {
				log.Println("Error reading from WebSocket:", err)
				break
			}

			if _, err := ptmx.Write(message); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to write to pty: %v\n", err)
				return
			}
		}
	}()

	_ = cmd.Wait()
}
