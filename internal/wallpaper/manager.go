package wallpaper

import (
	"bytes"
	"image"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/glxrender"
	"github.com/matjam/smoothpaper/internal/render"
	"github.com/spf13/viper"
)

type Manager struct {
	sync.Mutex
	wallpapers []string // list of wallpaper paths\
	renderer   render.Renderer
	exitSignal chan struct{}
}

func NewManager(wallpapers []string) *Manager {
	renderer, err := glxrender.NewRenderer(
		render.ScalingMode(viper.GetString("scale_mode")),
		render.EasingMode(viper.GetString("easing")),
		viper.GetInt("framerate_limit"),
	)
	if err != nil {
		log.Fatal("Failed to create wallpaper renderer:", err)
	}

	return &Manager{
		wallpapers: wallpapers,
		renderer:   renderer,
		exitSignal: make(chan struct{}, 1),
	}
}

func (c *Manager) Stop() {
	c.Lock()
	defer c.Unlock()

	if len(c.exitSignal) == 0 {
		c.exitSignal <- struct{}{}
	}
}

func (c *Manager) GetWallpapers() []string {
	c.Lock()
	defer c.Unlock()
	return c.wallpapers
}

func (c *Manager) SetWallpapers(wallpapers []string) {
	c.Lock()
	defer c.Unlock()
	c.wallpapers = wallpapers
}

func (c *Manager) NextWallpaper() string {
	c.Lock()
	defer c.Unlock()
	if len(c.wallpapers) == 0 {
		return ""
	}
	next := c.wallpapers[0]
	c.wallpapers = append(c.wallpapers[1:], next)
	return next
}

func (c *Manager) Shuffle() {
	c.Lock()
	defer c.Unlock()

	// Shuffle the wallpapers slice in place
	rand.Shuffle(len(c.wallpapers), func(i, j int) {
		c.wallpapers[i], c.wallpapers[j] = c.wallpapers[j], c.wallpapers[i]
	})
}

// Run will block until it receives a signal to stop
func (c *Manager) Run() {
	log.Info("Starting wallpaper changer...")

	timeChanged := time.Now()

	// Set the initial wallpaper
	c.Next()

	delay := viper.GetInt("delay")
	if delay == 0 {
		delay = 10
	}

	for {
		if len(c.exitSignal) > 0 {
			log.Info("Stopping wallpaper changer...")
			// read the value from the channel to clear it
			<-c.exitSignal
			break
		}

		if time.Since(timeChanged) > time.Duration(delay)*time.Second {
			log.Infof("Changing wallpaper after %d seconds", delay)
			c.Next()
			timeChanged = time.Now()
		}

		time.Sleep(500 * time.Millisecond)
	}

	c.renderer.Cleanup()
	log.Info("Wallpaper changer stopped.")
}

func (c *Manager) Next() {
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
	log.Infof("nextImg: %v x %v", nextImg.Bounds().Max.X, nextImg.Bounds().Max.Y)

	err = c.renderer.Transition(nextImg, time.Duration(viper.GetInt("fade_speed"))*time.Second)
	if err != nil {
		log.Fatal("Failed to transition images:", err)
	}

	log.Info("Image set/transition completed successfully.")
}
