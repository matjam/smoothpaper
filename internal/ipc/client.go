package ipc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"resty.dev/v3"
)

func SendNext() error {
	_, err := getRestyClient().R().Post("/next")
	return err
}

func SendStop() error {
	_, err := getRestyClient().R().Post("/stop")
	return err
}

func SendLoad(wallpapers []string) error {
	_, err := getRestyClient().R().
		SetBody(wallpapers).
		Post("/load")
	return err
}

func SendStatus() (*StatusResponse, error) {
	var status StatusResponse
	resp, err := getRestyClient().R().
		SetResult(&status).
		Get("/status")
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("status check failed: %s", resp.Status())
	}
	return &status, nil
}

func getRestyClient() *resty.Client {
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

	return client
}
