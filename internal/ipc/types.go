package ipc

type CommandType string

const (
	CommandStop   CommandType = "stop"   // stop the wallpaper manager
	CommandNext   CommandType = "next"   // next wallpaper
	CommandLoad   CommandType = "load"   // replace the list of wallpapers
	CommandStatus CommandType = "status" // gets the current status
	CommandAdd    CommandType = "add"
	CommandReload CommandType = "reload" // hotreload the config
)

type Command struct {
	Type  CommandType `json:"type"`
	Args  []string    `json:"args"`
	Reply chan error  `json:"-"` // if set, Run() sends the result of the command here
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

type ErrorResponse struct {
	Error string `json:"error"`
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
