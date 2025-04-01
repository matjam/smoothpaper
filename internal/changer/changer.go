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
	renderer     *xrender.WallpaperRenderer
	exitSignal   chan struct{}
}

func NewChanger(wallpapers []string) *Changer {
	renderer, err := xrender.NewWallpaperRenderer(xrender.FitMode(viper.GetString("scale_mode")))
	if err != nil {
		log.Fatal("Failed to create wallpaper renderer:", err)
	}

	return &Changer{
		wallpapers: wallpapers,
		renderer:   renderer,
		exitSignal: make(chan struct{}, 1),
	}
}

func (c *Changer) Stop() {
	c.Lock()
	defer c.Unlock()

	if len(c.exitSignal) == 0 {
		c.exitSignal <- struct{}{}
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
	log.Info("Starting wallpaper changer...")

	timeChanged := time.Now()

	// Set the initial wallpaper
	c.Next()

	for {
		if len(c.exitSignal) > 0 {
			log.Info("Stopping wallpaper changer...")
			// read the value from the channel to clear it
			<-c.exitSignal
			break
		}

		delay := viper.GetInt("delay")
		if delay == 0 {
			delay = 10
		}
		if time.Since(timeChanged) > time.Duration(delay)*time.Second {
			log.Infof("Changing wallpaper after %d seconds", delay)
			c.Next()
			timeChanged = time.Now()
		}

		// Sleep for a short duration to avoid busy waiting
		time.Sleep(33 * time.Millisecond)
		c.renderer.Renderer.RenderFade(1.0)
	}

	c.renderer.Cleanup()
	log.Info("Wallpaper changer stopped.")
}

func (c *Changer) Next() {
	// if the current image is nil, we set it to a black image
	if c.currentImage == nil {
		log.Infof(
			"currentImage is nil, creating a black image with width %v and height %v",
			c.renderer.Desktop.Width,
			c.renderer.Desktop.Height)

		newImage := image.NewRGBA(image.Rect(0, 0, 1, 1))
		for i := range 4 {
			newImage.Pix[i] = 0
		}
		c.currentImage = xrender.ScaleImage(newImage, c.renderer.Desktop.Width, c.renderer.Desktop.Height, xrender.FitStretch)
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

	err = c.renderer.Transition(c.currentImage, nextImg, 10*time.Second, xrender.EasingEaseInOut)
	if err != nil {
		log.Fatal("Failed to transition images:", err)
	}

	c.SetCurrentImage(nextImg)

	log.Info("Image transition completed successfully.")
}
