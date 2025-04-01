package ipc

import (
	"github.com/labstack/echo/v4"
)

func RegisterRoutes(e *echo.Echo, manager ManagerInterface) {
	e.GET("/status", statusHandler(manager))
	e.POST("/stop", stopHandler(manager))
	e.POST("/next", nextHandler(manager))
	e.POST("/load", loadHandler(manager))
}
