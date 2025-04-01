package wallpaper

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/matjam/smoothpaper"
	"github.com/matjam/smoothpaper/internal/middleware"
)

func StartSocketServer(m *Manager) {
	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = os.TempDir()
	}

	sockPath := sockDir + "/smoothpaper.sock"
	if _, err := os.Stat(sockPath); err == nil {
		os.Remove(sockPath)
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

	// Routes
	e.GET("/status", m.status)

	server := new(http.Server)
	if err := e.StartServer(server); err != nil {
		log.Fatal(err)
	}
}

// Handler
func (m *Manager) status(c echo.Context) error {
	return c.JSONPretty(http.StatusOK, map[string]any{
		"status":            "ok",
		"message":           "smoothpaper is running",
		"version":           strings.Trim(smoothpaper.Version, "\n\r "),
		"pid":               os.Getpid(),
		"socket":            os.Getenv("XDG_RUNTIME_DIR") + "/smoothpaper.sock",
		"config":            os.Getenv("XDG_CONFIG_HOME") + "/smoothpaper/config.yaml",
		"current_wallpaper": m.currentWallpaper,
	}, "  ")
}
