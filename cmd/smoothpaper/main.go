package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	"github.com/matjam/smoothpaper/internal/cli"
	"github.com/matjam/smoothpaper/internal/xrender"

	"github.com/spf13/viper"
)

func main() {

	os.ReadDir(viper.GetString("wallpapers"))

	cli.Execute()
}

func old() {
	rootWindow, err := xrender.GetRootWindow()
	if rootWindow == nil || err != nil {
		fmt.Println("Failed to get root window")
		os.Exit(1)
	}

	imgData, err := os.ReadFile("/home/matjam/Pictures/wallpapers/5120x1440wallpaper_51203074878_o.jpg")
	if err != nil {
		fmt.Println("Failed to read image file:", err)
		os.Exit(1)
	}
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		fmt.Println("Failed to decode image:", err)
		os.Exit(1)
	}
	if err := rootWindow.SetImage(img); err != nil {
		fmt.Println("Failed to set image:", err)
		os.Exit(1)
	}

	renderer, err := xrender.NewRenderer(rootWindow.Display, rootWindow.Window, 5120, 1440)
	if err != nil {
		fmt.Println("Failed to create renderer:", err)
		os.Exit(1)
	}

	nextImgData, err := os.ReadFile("/home/matjam/Pictures/wallpapers/5120x1440wallpaper_51202858196_o.jpg")
	if err != nil {
		fmt.Println("Failed to read next image file:", err)
		os.Exit(1)
	}
	nextImg, _, err := image.Decode(bytes.NewReader(nextImgData))
	if err != nil {
		fmt.Println("Failed to decode next image:", err)
		os.Exit(1)
	}

	err = renderer.LoadTextures(img, nextImg)
	if err != nil {
		fmt.Println("Failed to load textures:", err)
		os.Exit(1)
	}

	renderer.RenderFadeWithEasing(5*time.Second, xrender.EasingEaseInOut)

	err = rootWindow.SetImage(nextImg)
	if err != nil {
		fmt.Println("Failed to set next image:", err)
		os.Exit(1)
	}

	fmt.Println("Image transition completed successfully.")
}
