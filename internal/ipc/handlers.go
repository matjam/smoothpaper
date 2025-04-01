package ipc

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/matjam/smoothpaper"
)

func statusHandler(m ManagerInterface) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSONPretty(http.StatusOK, map[string]any{
			"status":            "ok",
			"message":           "smoothpaper is running",
			"version":           strings.Trim(smoothpaper.Version, "\n\r "),
			"pid":               os.Getpid(),
			"socket":            os.Getenv("XDG_RUNTIME_DIR") + "/smoothpaper.sock",
			"config":            os.Getenv("XDG_CONFIG_HOME") + "/smoothpaper/config.yaml",
			"current_wallpaper": m.CurrentWallpaper(),
		}, "  ")
	}
}

func commandHandler(m ManagerInterface) echo.HandlerFunc {
	return func(c echo.Context) error {
		var cmd Command
		if err := c.Bind(&cmd); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid command"})
		}
		m.EnqueueCommand(cmd)
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}
