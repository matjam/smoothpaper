package ipc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"resty.dev/v3"
)

func SendCommand(cmd Command) (*Response, error) {
	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = os.TempDir()
	}
	path := sockDir + "/smoothpaper.sock"

	client := resty.NewWithClient(&http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	})

	client.SetBaseURL("http://smoothpaper")
	client.SetHeader("Content-Type", "application/json")
	client.SetHeader("Accept", "application/json")
	client.SetHeader("User-Agent", "smoothpaper")

	result := Response{}

	response, err := client.R().SetBody(cmd).SetResult(&result).Post("/command")
	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("error sending command: %s", response.Status())
	}

	return &result, err
}
