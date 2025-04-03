package ipc

import (
	"bytes"
	"image"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/glxrenderer"
	"github.com/matjam/smoothpaper/internal/types"
	"github.com/matjam/smoothpaper/internal/wlrenderer"
	"github.com/spf13/viper"
)

type Manager struct {
	sync.Mutex
	wallpapers       []string // list of wallpaper paths\
	renderer         Renderer
	cmds             chan Command
	currentWallpaper string
}

// Renderer interface defines the methods that a renderer must implement to render
// images as wallpapers, including setting the current image, transitioning to the next
// image, rendering the current image, cleaning up resources, and getting the dimensions
// of the window.
type Renderer interface {
	SetImage(image image.Image) error                          // Set the current image
	Transition(next image.Image, duration time.Duration) error // Transition to the next image
	Render() error                                             // Render the current image, called in a loop and will block for each frame
	Cleanup()                                                  // Cleanup resources
	GetSize() (int, int)                                       // Get the dimensions of the window
	IsDisplayRunning() bool
	TryReconnect() error
}

// NewManager creates a new wallpaper manager with the specified wallpapers.
func NewManager(wallpapers []string) *Manager {
	var renderer Renderer
	var err error

	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		log.Info("Detected Wayland session")

		renderer, err = wlrenderer.NewRenderer(
			types.ScalingMode(viper.GetString("scale_mode")),
			types.EasingMode(viper.GetString("easing")),
			viper.GetInt("framerate_limit"),
		)
		if err != nil {
			log.Fatal("Failed to create wayland renderer:", err)
		}
	} else {
		log.Info("Wayland not detected, assuming X11 session")

		renderer, err = glxrenderer.NewRenderer(
			types.ScalingMode(viper.GetString("scale_mode")),
			types.EasingMode(viper.GetString("easing")),
			viper.GetInt("framerate_limit"),
		)
		if err != nil {
			log.Fatal("Failed to create glx renderer:", err)
		}
	}

	return &Manager{
		wallpapers: wallpapers,
		renderer:   renderer,
		cmds:       make(chan Command, 1),
	}
}

func (c *Manager) CurrentWallpaper() string {
	c.Lock()
	defer c.Unlock()
	return c.currentWallpaper
}

func (c *Manager) SetCurrentWallpaper(wallpaper string) {
	c.Lock()
	defer c.Unlock()
	c.currentWallpaper = wallpaper
}

func (c *Manager) Stop() {
	c.Lock()
	defer c.Unlock()

	if len(c.cmds) == 0 {
		c.cmds <- Command{
			Type: CommandStop,
			Args: []string{},
		}
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

	c.currentWallpaper = next

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
	c.NextWallpaper()
	c.SetCurrent()

	delay := viper.GetInt("delay")
	if delay == 0 {
		delay = 10
	}

	running := true

	for running {
		if len(c.cmds) > 0 {
			cmd := <-c.cmds
			switch cmd.Type {
			case CommandStop:
				log.Info("Stopping Wallpaper Manager ...")
				running = false
				continue
			case CommandNext:
				log.Info("Received next command")
				c.Next()
				timeChanged = time.Now()
			case CommandLoad:
				log.Info("Received load command")
				if len(cmd.Args) == 0 {
					log.Error("No wallpapers specified for load command")
					continue
				}
				c.SetWallpapers(cmd.Args)
				log.Infof("Loaded %d wallpapers", len(cmd.Args))
				c.Shuffle()
				c.Next()
				timeChanged = time.Now()
			default:
				log.Error("Unknown command:", cmd.Type)
			}
		} else if time.Since(timeChanged) > time.Duration(delay)*time.Second {
			c.Next()
			timeChanged = time.Now()
		}

		// Update the image so if the X server goes away and comes back the wallpaper
		// will be set again
		err := c.renderer.Render()
		if err != nil {
			log.Error("renderer.Render() failed:", err)
		}

		time.Sleep(500 * time.Millisecond)

		if !c.renderer.IsDisplayRunning() {
			log.Info("Display connection lost, attempting to reconnect...")
			time.Sleep(1 * time.Second) // Wait a bit before reconnecting
			for {
				err = c.renderer.TryReconnect()
				if err == nil {
					log.Info("Display connection re-established")
					break
				}
				log.Debug("Failed to reconnect to display:", err)
				time.Sleep(1 * time.Second) // Wait a bit before retrying
			}
			c.SetCurrent()
			timeChanged = time.Now()
		}
	}

	c.renderer.Cleanup()
	log.Info("Wallpaper Manager stopped.")
}

func (c *Manager) Next() {
	nextFile := c.NextWallpaper()
	if nextFile == "" {
		log.Fatal("No next wallpaper found")
		os.Exit(1)
	}

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
	log.Infof("loading %v (%vx%v)", nextFile, nextImg.Bounds().Max.X, nextImg.Bounds().Max.Y)

	err = c.renderer.Transition(nextImg, time.Duration(viper.GetInt("fade_speed"))*time.Second)
	if err != nil {
		log.Errorf("Failed to transition images: %v", err)
	}
}

func (c *Manager) SetCurrent() {
	log.Infof("Setting current wallpaper: %s", c.CurrentWallpaper())
	imgData, err := os.ReadFile(c.CurrentWallpaper())
	if err != nil {
		log.Error("Failed to read current image file:", err)
		return
	}
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		log.Error("Failed to decode current image:", err)
		return
	}
	err = c.renderer.SetImage(img)
	if err != nil {
		log.Error("Failed to set current image:", err)
		return
	}
	log.Infof("Successfully set current wallpaper: %s", c.CurrentWallpaper())
	c.renderer.Render()
}

func (c *Manager) EnqueueCommand(cmd Command) {
	c.Lock()
	defer c.Unlock()

	c.cmds <- cmd
}
