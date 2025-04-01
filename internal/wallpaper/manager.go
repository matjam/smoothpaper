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

type CommandType string

const (
	CommandStop CommandType = "stop"
	CommandNext CommandType = "next"
	CommandLoad CommandType = "load"
)

// Command is used to send commands to the wallpaper manager
// via the command channel. The command is a string that specifies
// the type of command to execute, and args is a slice of strings
// that contains the arguments for the command.
type Command struct {
	Command CommandType `json:"type"`
	Args    []string    `json:"args"`
}

type Manager struct {
	sync.Mutex
	wallpapers       []string // list of wallpaper paths\
	renderer         render.Renderer
	cmds             chan Command
	currentWallpaper string
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
		cmds:       make(chan Command, 1),
	}
}

func (c *Manager) Stop() {
	c.Lock()
	defer c.Unlock()

	if len(c.cmds) == 0 {
		c.cmds <- Command{
			Command: CommandStop,
			Args:    []string{},
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
	c.Next()

	delay := viper.GetInt("delay")
	if delay == 0 {
		delay = 10
	}

	running := true

	for running {
		if len(c.cmds) > 0 {
			cmd := <-c.cmds
			switch cmd.Command {
			case CommandStop:
				log.Info("Stopping wallpaper changer...")
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
				log.Error("Unknown command:", cmd.Command)
			}
		} else if time.Since(timeChanged) > time.Duration(delay)*time.Second {
			c.Next()
			timeChanged = time.Now()
		}

		time.Sleep(500 * time.Millisecond)

		if !c.renderer.IsDisplayRunning() {
			log.Info("Display is not running, stopping wallpaper changer...")
			running = false
			continue
		}
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
		log.Fatal("Failed to transition images:", err)
	}
}
