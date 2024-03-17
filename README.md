# smoothpaper

Smoothly transitioning wallpaper daemon for X11 Window Mangers.

https://github.com/matjam/smoothpaper/assets/578676/152657d8-e321-4aa9-8e3c-8d3c2eb1fcad

Smoothpaper is a SFML based wallpaper slideshow daemon designed to work with  X11 Window Managers, such as Qtile, i3, Openbox, etc.

It currently can't set the desktop wallpaper of any DE that sets its own wallpaper (like Gnome, KDE, etc).

Its been tested with Qtile, i3, and Openbox, but should work with any X11 WM that doesn't manage the desktop wallpaper.

In my travels from Windows/Mac to using tiling window managers on Linux, I was frustrated with the lack of any way to have a set of wallpapers that transition smoothly while I work. I was using feh and nitrogen to set wallpapers, but the transitions were jarring. I wanted smooth fading between the wallpapers, and couldn't find a program that did this for X11 (I am aware there are programs that do this for Wayland), so I wrote one.

You _can_ use xscreensaver's glslideshow to transition between wallpapers, but it's not as smooth as smoothpaper. 

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
- Can be run as a daemon with the `-b` flag

### TODO

- Dynamically change the wallpappers directory (currently you have to restart the program to change the directory
- Set a specific wallpaper immediately via command line without waiting for the next transition

## Limitations

Does not work on multi-monitor setups as it currently finds the first screen and sets the wallpaper for that screen only. Should be easy to fix, but I don't have a multi-monitor setup to test with.

## Building

You will need to install vcpkg to build this project. You can find instructions on how to install vcpkg here: https://github.com/microsoft/vcpkg

I recommend installing vcpkg in your home directory, but you can install it anywhere you like. I have it installed in `~/.local/share/vcpkg`. You will then need to set `VCPKG_ROOT` to the directory you installed vcpkg in. For example, you can place this in your `.bashrc` or `.zshrc`:

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
cmake .. -DCMAKE_TOOLCHAIN_FILE=$VCPKG_ROOT/scripts/buildsystems/vcpkg.cmake -preset=RELEASE
cd RELEASE
make
```

This will output the `smoothpaper` binary in the `build/RELEASE` directory. 

## Installation

Copy `smoothpaper` anywhere in your path, or run from the build directory. 

Copy the `smoothpaper.toml` file to `~/.config/smoothpaper/smoothpaper.toml` and edit it to your liking.

## Usage

Simply run the `smoothpaper` binary. It will read the configuration file and set the wallpaper for the first screen it finds. It will then transition to the next wallpaper after the delay specified in the configuration file.

Smoothpaper supports daemonizing with the `-b` flag. This will run the program in the background and will not print any output to the terminal. Logs will be written to `~/.local/share/smoothpaper/smoothpaper.log`, and will be rotated when the log file reaches 1MB in size, with 3 backups.

```bash
smoothpaper -b
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
