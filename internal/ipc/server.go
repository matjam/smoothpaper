package ipc

import (
	"net"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/matjam/smoothpaper/internal/middleware"
)

func Start(manager ManagerInterface) {
	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = os.TempDir()
	}
	sockPath := sockDir + "/smoothpaper.sock"

	if _, err := os.Stat(sockPath); err == nil {
		_ = os.Remove(sockPath)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Listener = listener

	e.Use(middleware.CharmLog())

	RegisterRoutes(e, manager)

	server := new(http.Server)
	if err := e.StartServer(server); err != nil {
		log.Fatalf("Socket server error: %v", err)
	}
}
