package ipc

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/matjam/smoothpaper"
	"github.com/spf13/viper"
)

// GET /status
func statusHandler(m ManagerInterface) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSONPretty(http.StatusOK, StatusResponse{
			Status:           "ok",
			Message:          "smoothpaper is running",
			Version:          strings.Trim(smoothpaper.Version, "\n\r "),
			PID:              os.Getpid(),
			Socket:           os.Getenv("XDG_RUNTIME_DIR") + "/smoothpaper.sock",
			Config:           viper.ConfigFileUsed(),
			CurrentWallpaper: m.CurrentWallpaper(),
		}, "  ")
	}
}

// POST /stop
func stopHandler(m ManagerInterface) echo.HandlerFunc {
	return func(c echo.Context) error {
		m.EnqueueCommand(Command{Type: CommandStop})
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// POST /next
func nextHandler(m ManagerInterface) echo.HandlerFunc {
	return func(c echo.Context) error {
		m.EnqueueCommand(Command{Type: CommandNext})
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// POST /load
func loadHandler(m ManagerInterface) echo.HandlerFunc {
	return func(c echo.Context) error {
		var wallpapers []string
		if err := c.Bind(&wallpapers); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON array of wallpapers"})
		}

		m.EnqueueCommand(Command{
			Type: CommandLoad,
			Args: wallpapers,
		})

		return c.JSON(http.StatusOK, map[string]any{
			"status": "ok",
			"loaded": len(wallpapers),
		})
	}
}
