# smoothpaper

Smoothly transitioning wallpaper daemon for X11 Window Mangers.

https://github.com/matjam/smoothpaper/assets/578676/152657d8-e321-4aa9-8e3c-8d3c2eb1fcad

Smoothpaper is designed to work only with X11 Window Managers and currently can't set the desktop wallpaper of any DE that sets its own wallpaper (like Gnome, KDE, etc).

Its been tested with Qtile, i3, and Openbox, but should work with any X11 WM that doesn't manage the desktop wallpaper.

The main use-case for this program is to have a smooth transition between wallpapers on a multi-monitor setup. I was using feh and nitrogen to set wallpapers, but the transitions were jarring and not smooth. I wanted a smooth transition between wallpapers and couldn't find a program that did this, so I wrote one.

You _can_ use xscreensaver's glslideshow to transition between wallpapers, but it's not as smooth as smoothpaper and doesn't have the same features.

Because I'm using SFML to do the transitions, I can use a Texture that is stored on the GPU and the transitions are done by changing the alpha value of the texture, so the transitions are very smooth and use very little CPU.

## Features

- Smoothly transition between wallpapers with fading
- Set wallpapers from a directory
- Scaling of images to fit the screen
- Randomly select wallpapers
- Set the time between transitions
- Set the speed of fade transitions
- Uses SFML for smooth transitions
- Uses very little CPU when idle

### TODO

- Can be run as a daemon
- Dynamically change the wallpappers directory (currently you have to restart the program to change the directory
- Set a specific wallpaper immediately via command line without waiting for the next transition

## Limitations

Does not work on multi-monitor setups as it currently finds the first screen and sets the wallpaper for that screen only. Should be easy to fix, but I don't have a multi-monitor setup to test with.

## Building

You will need to install vcpkg to build this project. You can find instructions on how to install vcpkg here: https://github.com/microsoft/vcpkg

I recommedn installing vcpkg in your home directory, but you can install it anywhere you like. I have it installed in `~/.local/share/vcpkg`. You will then need to set `VCPKG_ROOT` to the directory you installed vcpkg in. For example, you can place this in your `.bashrc` or `.zshrc`:

```bash
export VCPKG_ROOT=/home/matjam/.local/share/vcpkg
```

Additionally, you will need to install gcc, cmake, and make, as well as the X11 development libraries. On Ubuntu/Debian, you can install these with the following command:

```bash
sudo apt install build-essential cmake libx11-dev libxrandr-dev libxinerama-dev libxcursor-dev libxext-dev
```

On Arch Linux, you can install these with the following command:

```bash
sudo pacman -S base-devel cmake libx11 libxrandr libxinerama libxcursor libxext
```

I use cmake to build the project. You can build it with the following commands:

```bash
mkdir build
cd build
cmake ..
make
```

This will output a binary in the `build/default` directory. You can copy it to your path or run it from the build directory. At some point I'll add an install target to the cmake file.

## Usage

```bash
Wallpaper changer with smooth transitions for X11 Window Managers.
Usage:
  smoothpaper [OPTION...]

  -d, --debug    Enable debug logging
  -h, --help     Print usage
  -v, --version  Print version
```

## Configuration

The program looks for a configuration file in `~/.config/smoothpaper/smoothpaper.toml`. An example configuration file is below:

```toml
# smoothpaper configuration file

# path your wallpapers are stored in
wallpapers = "~/Pictures"

# whether the files should be shuffled or not. If you do not shuffle, the files will be 
# displayed in the order they are found which is dependent on the filesystem.
shuffle = true

# the mode to scale the images. Options are
#
#   "vertical": scales the image to fit vertically; on widescreens this might result in
#               bars on the side if the wallpaper has a less wide aspect ratio. On other
#               displays this might mean cropping the sides if the wallpaper has a wider
#               aspect ratio than the screen.
#
# "horizontal": scales the image to fit horizintally; on widescreens this might result in
#               cropping the top and bottom of the image. On a 4:3 or 16:9 screen this might
#               result in bars on the top and bottom if the aspect ratio of the wallpaper
#               is wider than your screen.
#
#  "stretched": will stretch the image in both directions to fit your screen. This means
#               that if the image does not match your screen's aspect ratio, it will be
#               distorted.
scale_mode = "vertical"

# the speed at which the images fade in and out, in seconds. This may be fractional.
fade_speed = 1.0

# the delay between images, in seconds. Must be an integer.
delay = 300

# SFML's window redraw framerate limit. This is the maximum number of frames per second 
# the window will redraw at. Higher numbers will be smoother, but will use more CPU during
# fade in of a new wallpaper. smoothpaper sleeps between transitions so should not use any
# CPU when idle.
framerate_limit = 60

# whether to display debug information or not.
debug = false
```

# Bugs

Please use the github Issues to report any bugs or feature requests. I'm happy to accept pull requests for new features or bug fixes, and in fact prefer that. I'm not a C++ programmer by trade, so I'm sure there are many things that could be improved.

# License

This program is licensed under the Apache 2.0 license. See the LICENSE file for more information.
