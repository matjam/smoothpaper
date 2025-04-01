package ipc

type CommandType string

const (
	CommandStop   CommandType = "stop"   // stop the wallpaper manager
	CommandNext   CommandType = "next"   // next wallpaper
	CommandLoad   CommandType = "load"   // replace the list of wallpapers
	CommandStatus CommandType = "status" // gets the current status
	CommandAdd    CommandType = "add"
)

type Command struct {
	Type CommandType `json:"type"`
	Args []string    `json:"args"`
}

type ManagerInterface interface {
	CurrentWallpaper() string
	EnqueueCommand(Command)
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type StatusResponse struct {
	Status           string `json:"status"`
	Message          string `json:"message"`
	Version          string `json:"version"`
	PID              int    `json:"pid"`
	Socket           string `json:"socket"`
	Config           string `json:"config"`
	CurrentWallpaper string `json:"current_wallpaper"`
}
