package main

import (
	// import image formats to register them
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/matjam/smoothpaper/internal/cli"
)

func main() {
	cli.Execute()
}
