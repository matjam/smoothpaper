# Smoothpaper

Smoothly transitioning wallpaper daemon for X11 Window Mangers and Wayland
Compositors.

https://github.com/matjam/smoothpaper/assets/578676/152657d8-e321-4aa9-8e3c-8d3c2eb1fcad

Smoothpaper is an OpenGL based wallpaper slideshow daemon designed to work with
X11 Window Managers, such as Qtile, i3, Openbox, etc, as well as Wayland
Compositors such as Hyprland. There are many implementations of this kind of
utility, and arguably its already well served by the existing tools, but I
switch between X11 and Wayland a fair bit due to compatibility issues, so I
wanted something I could just use in both environments.

It currently can't set the desktop wallpaper of any DE that sets its own
wallpaper (like Gnome, KDE, etc), as those applications open their own Window to
render the desktop icons to and have their own mangement of the wallpaper. KDE
and Gnome both have extensions or built-in configuration for doing what I do in
smoothpaper, so I don't see much value in implementing that.

Its been tested with i3, Hyprland and Sway. Most tiling window managers should
be fine, but I noticed issues with Openbox, which I will look at. But for now
things work ok.

In my travels from Windows/Mac to using tiling window managers on Linux, I was
frustrated with the lack of any way to have a set of wallpapers that transition
smoothly while I work. I was using feh and nitrogen to set wallpapers, but the
transitions were jarring. I wanted smooth fading between the wallpapers, and
couldn't find a program that did this for X11 (I am aware there are programs
that do this for Wayland), so I wrote one. And then, I decided that I could add
Wayland support so I did that too.

Because I'm using OpenGL to do the transitions, I can use a Texture that is
stored on the GPU and the transitions are done by changing the alpha value of
the texture, so the transitions are very smooth and use very little CPU.

## 2.0 Release

So, I was pretty annoyed at how difficult adding certain features was to the C++
version of the application, so I rewrote it in Go using cgo. This took a couple
of days, but I think it was worth it as I now have a clean backend that made it
easy to implement Wayland support.

The program has less exotic dependencies; standard OpenGL, X11 and Wayland
libraries you most likely already have installed.

So, many of the items on the todo list are much closer now to actually getting
done. If you want a feature, let me know!

Note one major change is I do not support running in the background. This is the
one thing thats super hard in Go, but I don't think its necessary as you can
start it and background it just fine with `&` and most Window Managers allow you
to start things with `exec` for example, in `i3`. If it really annoys you, let
me know and I'll spend some time on it; its not impossible just irksome.

## Features

- Smoothly transition between wallpapers with fading
- Set wallpapers from one or more directories
- Scaling of images to fit the screen
- Randomly select wallpapers
- Set the time between transitions
- Set the speed of fade transitions
- Uses SFML for smooth transitions
- Uses very little CPU when idle
- Can be run as a daemon with the `-b` flag
- Set a specific wallpaper immediately via command line without waiting for the
  next transition
- Add cli commands to control the program

## Known Issues

Please check the Issues in Github for all issues. Major ones are:

- Currently does not necessarily exit when the compositor or x11 shuts down. You
  might need to issue a `smoothpaper stop` before you start it in case there's a
  stale one running in the background. Your other option is to not use -b; if
  the parent process dies in that case it should remove the daemon.

### TODO

- Dynamically change the wallpapers directory (currently you have to restart the
  program to change the directory)
- Set the wallpaper for all screens in a multi-monitor setup
- Add a task bar icon to control the program
- More cool transitions?

If you have any feature requests, please open an issue.

## Limitations

- Does not work on multi-monitor setups as it currently finds the first screen
  and sets the wallpaper for that screen only. Should be easy to fix, but I
  don't have a multi-monitor setup to test with.
- Any X11 Window Manager that renders to the desktop will have problems. I think
  its because I am creating a layer just above the bottom layer or something
  like that.
- Wayland works, but I don't know how stable it is. Appreciate any feedback if
  you have issues.

## Installation

### Arch Linux

Smoothpaper is available in the AUR as `smoothpaper`. You can install it with
your favorite AUR helper, such as `yay`:

```bash
yay -S smoothpaper
```

Please make sure you raise any issues installing the AUR package in this
repository, not the AUR, so I can track them.

### Other Distributions

I don't have packages for other distributions yet. If you would like to package
smoothpaper for your distribution, please let me know and I can help you with
that.

## Building

You will need to install g 1.24.1 to build this project. Go is included in the
Arch repositories, and you can also just download a tarball and shove it in your
path from https://go.dev/doc/install.

Additionally, you will need to install gcc, cmake, and make, as well as the X11
development libraries and any OpenGL lib like mesa that you need. On
Ubuntu/Debian, you can install these with the following command:

```bash
sudo apt install build-essential cmake libx11-dev libxrandr-dev libxinerama-dev \
                 libxcursor-dev libxext-dev libgl-dev
```

(I'm unsure if the above is correct, I'll check but if you get build failures,
check the output and let me know in a new issue and I'll update the docs)

On Arch Linux, you can install these with the following command:

```bash
sudo pacman -S base-devel go mesa glad libxrender libva wayland egl-wayland
```

You should then be able to do

```bash
go install ./...
```

This will output the `smoothpaper` binary in your `~/go/bin` folder. You can add
that to your path. If it complains about missing dependencies etc, let me know
and I'll figure out what I am missing from the documentation.

## Installation

Copy `smoothpaper` anywhere in your path, or run from the build directory.

Copy the `smoothpaper.toml` file to `~/.config/smoothpaper/smoothpaper.toml` and
edit it to your liking, or use `smoothpaper -i` to install the default config
and edit that.

# Usage

Simply run the `smoothpaper` binary. It will read the configuration file and set
the wallpaper for the first screen it finds. It will then transition to the next
wallpaper after the delay specified in the configuration file.

Smoothpaper supports daemonizing with the `-b` flag. This will run the program
in the background and will not print any output to the terminal after the
initial message. Logs will be written to
`~/.local/share/smoothpaper/smoothpaper.log`, and will be rotated when the log
file reaches 1MB in size, with 3 backups.

```bash
smoothpaper -b
```

## Configuration

The program looks for a configuration file in
`~/.config/smoothpaper/smoothpaper.toml`. An example configuration file is
below:

```toml
# smoothpaper configuration file

# path your wallpapers are stored in
wallpapers = "~/Pictures"
# alternatively, you can specify a list of directories to search for wallpapers
# wallpapers = ["~/Pictures/wallpapers", "~/Downloads/wide_walls"]

# whether the files should be shuffled or not. If you do not shuffle, the files will be
# displayed in the order they are found which is dependent on the filesystem. I'm not
# sure why you'd want to do this, but it's an option.
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
#
#     "center": will center the image on the screen. This means that if the image does not match
#               your screen's aspect ratio, it will be cropped on the sides or top and bottom.
#               depending on the mode you choose, or it might just be centered with bars on the sides
#               or top and bottom.
scale_mode = "horizontal"

# the speed at which the images fade in and out, in seconds.
fade_speed = 5

# the delay between images, in seconds. Must be an integer.
delay = 300

# frames per second for the opengl renderer. This is the maximum number of frames per second
# that will be rendered. Lowering this value will reduce CPU usage, but may cause the animation
# to be less smooth.
framerate_limit = 60

# whether to display debug information or not.
debug = false
```

## CLI

Running `smoothpaper` by itself will output some help. You can control a running
smoothpaper daemon by using the following commands:

- `smoothpaper next` - switch to the next wallpaper
- `smoothpaper load <filename>` - switch to the given wallpaper. You must give
  an absolute path.
- `smoothpaper status` - returns the currently shown wallpaper and the status of
  the daemon, in JSON format.
- `smoothpaper stop` - exits the daemon.

The following switches are supported for the `smoothpaper` command:

```
  -c, --config arg     Path to config file
  -i, --installconfig  Install a default config file
  -d, --debug          Enable debug logging
  -h, --help           Print usage
  -v, --version        Print version
```

They should be mostly self explanatory; the `--installconfig` flag will install
config to `$HOME/.config/smoothpaper/smoothpaper.toml`.

Use the `--help` or look at the man pages for more detailed information.

# Support

Please use the github Issues to report any bugs or feature requests. I'm happy
to accept pull requests for new features or bug fixes, and in fact prefer that.
I'm a go programmer by trade, so I have no excuses.

You can also find me as `matjammer` on Discord. One server I frequent is the
Hyprland Cathedral https://hyprland.org/

## Contributing

If you would like to contribute to smoothpaper, please open a pull request. I'm
happy to accept any contributions, and will work with you to get your changes
merged. I am a Go programmer by trade, but the C code that I'm integrating with
is entirely alien to me so its quite likely I've missed something. LLMs are
helpful in this regard, but they do make mistakes.

## Thanks

- vee on libera.chat for beta testing and feature requests
- @strongleong for suggesting being able to control the daemon via a cli tool
  (in progress!)

## License

This program is licensed under the MIT License. See the LICENSE file for more
information.
