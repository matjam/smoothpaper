package ipc

type CommandType string

const (
	CommandStop   CommandType = "stop"
	CommandNext   CommandType = "next"
	CommandLoad   CommandType = "load"
	CommandStatus CommandType = "status"
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
