# Smoothpaper

Smoothly transitioning wallpaper daemon for X11 Window Mangers.

https://github.com/matjam/smoothpaper/assets/578676/152657d8-e321-4aa9-8e3c-8d3c2eb1fcad

Smoothpaper is an OpenGL based wallpaper slideshow daemon designed to work with X11 Window Managers, such as Qtile, i3,
Openbox, etc.

It currently can't set the desktop wallpaper of any DE that sets its own wallpaper (like Gnome, KDE, etc), as those
applications open their own Window to render the desktop icons to and have their own mangement of the wallpaper. KDE and
Gnome both have extensions or built-in configuration for doing what I do in smoothpaper, so I don't see much value in
implementing that.

Its been tested with Qtile, i3, and Openbox, but should work with any X11 WM that doesn't manage the desktop wallpaper.

In my travels from Windows/Mac to using tiling window managers on Linux, I was frustrated with the lack of any way to
have a set of wallpapers that transition smoothly while I work. I was using feh and nitrogen to set wallpapers, but the
transitions were jarring. I wanted smooth fading between the wallpapers, and couldn't find a program that did this for
X11 (I am aware there are programs that do this for Wayland), so I wrote one.

You _can_ use xscreensaver's glslideshow to transition between wallpapers, but it's not as smooth as smoothpaper, and
requires the xwinwrap program to work, which is a bit of a hack.

Because I'm using OpenGL to do the transitions, I can use a Texture that is stored on the GPU and the transitions are
done by changing the alpha value of the texture, so the transitions are very smooth and use very little CPU.

## 2.0 Release

So, I was pretty annoyed at how difficult adding certain features was to the C++ version of the application, so I
rewrote it in Go using cgo. This took a couple of days, but I think it was worth it as I now have a clean backend that I
can also use to implement support for Wayland in the near future.

The program has less exotic dependencies; standard X11 libraries you probably already have installed, no need for SFML
and all the baggage it brings.

I was even able to implement the same rendering that we had with SFML so it works the same both under a compositor like
picom, as well as without one.

So, many of the items on the todo list are much closer now to actually getting done. If you want a feature, let me know!

Note one major change is I do not support running in the background. This is the one thing thats super hard in Go, but I
don't think its necessary as you can start it and background it just fine with `&` and most Window Managers allow you to
start things with `exec` for example, in `i3`. If it really annoys you, let me know and I'll spend some time on it; its
not impossible just irksome.

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

- Dynamically change the wallpappers directory (currently you have to restart the program to change the directory)
- Set a specific wallpaper immediately via command line without waiting for the next transition
- Set the wallpaper for all screens in a multi-monitor setup
- Add a task bar icon to control the program
- Add cli commands to control the program
- More cool transitions?

If you have any feature requests, please open an issue.

## Limitations

Does not work on multi-monitor setups as it currently finds the first screen and sets the wallpaper for that screen
only. Should be easy to fix, but I don't have a multi-monitor setup to test with.

Obviously, it only works with X11 window managers, and not Wayland yet. Wayland support is planned.

## Installation

### Arch Linux

Smoothpaper is available in the AUR as `smoothpaper`. You can install it with your favorite AUR helper, such as `yay`:

```bash
yay -S smoothpaper
```

Please make sure you raise any issues installing the AUR package in this repository, not the AUR, so I can track them.

### Other Distributions

I don't have packages for other distributions yet. If you would like to package smoothpaper for your distribution,
please let me know and I can help you with that.

## Building from Source

You will need to install g 1.24.1 to build this project. Go is included in the Arch repositories, and you can also just
download a tarball and shove it in your path from https://go.dev/doc/install.

Additionally, you will need to install gcc, cmake, and make, as well as the X11 development libraries and any OpenGL lib
like mesa that you need. On Ubuntu/Debian, you can install these with the following command:

```bash
sudo apt install build-essential cmake libx11-dev libxrandr-dev libxinerama-dev libxcursor-dev libxext-dev libgl-dev
```

(I'm unsure if the above is correct, I'll check but if you get build failures, check the output and let me know in a new
issue and I'll update the docs)

On Arch Linux, you can install these with the following command:

```bash
sudo pacman -S base-devel go mesa glad libxrender libva
```

You should then be able to do

```bash
go install ./...
```

This will output the `smoothpaper` binary in your `~/go/bin` folder. You can add that to your path.

## Installation

Copy `smoothpaper` anywhere in your path, or run from the build directory.

Copy the `smoothpaper.toml` file to `~/.config/smoothpaper/smoothpaper.toml` and edit it to your liking, or use
`smoothpaper -i` to install the default config.

## Usage

Simply run the `smoothpaper` binary. It will read the configuration file and set the wallpaper for the first screen it
finds. It will then transition to the next wallpaper after the delay specified in the configuration file.

### Currently not working:

Smoothpaper supports daemonizing with the `-b` flag. This will run the program in the background and will not print any
output to the terminal. Logs will be written to `~/.local/share/smoothpaper/smoothpaper.log`, and will be rotated when
the log file reaches 1MB in size, with 3 backups.

```bash
smoothpaper -b
```

TODO: Will get this working again, its just getting late and I'm tired.

## Configuration

The program looks for a configuration file in `~/.config/smoothpaper/smoothpaper.toml`. An example configuration file is
below:

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

## CLI - TODO

Running `smoothpaper` by itself will output some help. You can control a running smoothpaper daemon by using the
following commands:

- `smoothpaper next` - switch to the next wallpaper
- `smoothpaper show <filename>` - switch to the given wallpaper. You can give an absolute path, or a file that exists in
  the wallpapers path you have configured.
- `smoothpaper info` - returns the currently shown wallpaper, and the next wallpaper, as well as the time until it will
  switch. Use `-j` if you want the output in JSON format.
- `smoothpaper quit` - exits the daemon.

The following switches are supported for the `smoothpaper` command:

```
  -c, --config arg     Path to config file
  -i, --installconfig  Install a default config file
  -d, --debug          Enable debug logging
  -h, --help           Print usage
  -v, --version        Print version
```

They should be mostly self explanatory; the `--installconfig` flag will install config to
`$HOME/.config/smoothpaper/smoothpaper.toml`.

## Support

Please use the github Issues to report any bugs or feature requests. I'm happy to accept pull requests for new features
or bug fixes, and in fact prefer that. I'm a go programmer by trade, so I have no excuses.

You can also find me as `matjammer` on Discord. One server I frequent is the Hyprland Cathedral https://hyprland.org/

## Contributing

If you would like to contribute to smoothpaper, please open a pull request. I'm happy to accept any contributions, and
will work with you to get your changes merged. I'm not a C++ programmer by trade, so I'm sure there are many things that
could be improved. I'm especially interested in any contributions that implement items in the TODO list above.

## Thanks

- vee on libera.chat for beta testing and feature requests
- @strongleong for suggesting being able to control the daemon via a cli tool (in progress!)

## License

This program is licensed under the Apache 2.0 license. See the LICENSE file for more information.
