package changer

import (
	"bytes"
	"image"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/xrender"
	"github.com/spf13/viper"
)

type Changer struct {
	sync.Mutex
	wallpapers   []string // list of wallpaper paths\
	currentImage image.Image
	rootWindow   *xrender.RootWindow

	exitSignal chan struct{}
}

func NewChanger(wallpapers []string) *Changer {
	rootWindow, err := xrender.GetRootWindow()
	if rootWindow == nil || err != nil {
		log.Fatal("Failed to get root window")
	}

	return &Changer{
		wallpapers: wallpapers,
		rootWindow: rootWindow,
		exitSignal: make(chan struct{}, 1),
	}
}
func (c *Changer) GetCurrentImage() image.Image {
	c.Lock()
	defer c.Unlock()
	return c.currentImage
}

func (c *Changer) SetCurrentImage(img image.Image) {
	c.Lock()
	defer c.Unlock()
	c.currentImage = img
}

func (c *Changer) GetWallpapers() []string {
	c.Lock()
	defer c.Unlock()
	return c.wallpapers
}

func (c *Changer) SetWallpapers(wallpapers []string) {
	c.Lock()
	defer c.Unlock()
	c.wallpapers = wallpapers
}

func (c *Changer) NextWallpaper() string {
	c.Lock()
	defer c.Unlock()
	if len(c.wallpapers) == 0 {
		return ""
	}
	next := c.wallpapers[0]
	c.wallpapers = append(c.wallpapers[1:], next)
	return next
}

func (c *Changer) Shuffle() {
	c.Lock()
	defer c.Unlock()

	// Shuffle the wallpapers slice in place
	rand.Shuffle(len(c.wallpapers), func(i, j int) {
		c.wallpapers[i], c.wallpapers[j] = c.wallpapers[j], c.wallpapers[i]
	})
}

// Run will block until it receives a signal to stop
func (c *Changer) Run() {
	for len(c.exitSignal) == 0 {
		c.Next()
		time.Sleep(10 * time.Second)
	}
}

func (c *Changer) Next() {
	// if the current image is nil, we set it to a black image
	if c.currentImage == nil {
		log.Infof("currentImage is nil, creating a black image with width %v and height %v", c.rootWindow.Width, c.rootWindow.Height)
		c.currentImage = image.NewRGBA(image.Rect(0, 0, int(c.rootWindow.Width), int(c.rootWindow.Height)))
	}

	renderer, err := xrender.NewRenderer(c.rootWindow.Display, c.rootWindow.Window, int(c.rootWindow.Width), int(c.rootWindow.Height))
	if err != nil {
		log.Fatal("Failed to create renderer:", err)
		os.Exit(1)
	}

	nextFile := c.NextWallpaper()
	if nextFile == "" {
		log.Fatal("No next wallpaper found")
		os.Exit(1)
	}
	log.Infof("Next wallpaper: %s", nextFile)

	nextImgData, err := os.ReadFile(nextFile)
	if err != nil {
		log.Fatal("Failed to read next image file:", err)
		os.Exit(1)
	}
	nextImg, _, err := image.Decode(bytes.NewReader(nextImgData))
	if err != nil {
		log.Fatal("Failed to decode next image:", err)
		os.Exit(1)
	}

	log.Infof("currentImage: %v", c.currentImage.Bounds())
	log.Infof("nextImg: %v", nextImg.Bounds())
	log.Infof("Loading textures...")

	err = renderer.LoadTextures(c.currentImage, nextImg)
	if err != nil {
		log.Fatal("Failed to load textures:", err)
		os.Exit(1)
	}

	renderer.RenderFadeWithEasing(viper.GetDuration("fade_speed")*time.Second, xrender.EasingMode(viper.GetString("easing")))

	err = c.rootWindow.SetImage(nextImg)
	if err != nil {
		log.Fatal("Failed to set next image:", err)
		os.Exit(1)
	}

	c.SetCurrentImage(nextImg)

	log.Info("Image transition completed successfully.")
}
