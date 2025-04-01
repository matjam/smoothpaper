package command

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"github.com/tidwall/pretty"
	"resty.dev/v3"
)

func GetStatus() error {
	sockDir := os.Getenv("XDG_RUNTIME_DIR")
	if sockDir == "" {
		sockDir = os.TempDir()
	}

	path := sockDir + "/smoothpaper.sock"

	socketClient := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", path)
			},
		},
	}

	client := resty.NewWithClient(&socketClient)
	client.SetBaseURL("http://smoothpaper")
	client.SetHeader("Content-Type", "application/json")
	client.SetHeader("Accept", "application/json")
	client.SetHeader("User-Agent", "smoothpaper")

	res, err := client.R().Get("/status")
	if err != nil {
		return fmt.Errorf("error pinging socket: %w", err)
	}
	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("error pinging socket: %s", res.Status())
	}

	printResponseBody(res)

	return nil
}

func printResponseBody(res *resty.Response) {
	jPretty := pretty.Color(res.Bytes(), nil)
	log.Info(string(jPretty))
}
