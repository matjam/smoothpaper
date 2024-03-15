#pragma once

#include <deque>
#include <filesystem>

#include <cxxopts.hpp>
#include <toml.hpp>

enum class WallpaperScaleMode {
  HorizontalFit,
  VerticalFit,
  StretchedFit,
};

class Config {
private:
  toml::value                     _config;
  std::filesystem::file_time_type _last_write_time;
  std::string                     _loaded_config_path;

  bool               _debug;
  float              _fade_speed;
  unsigned int       _framerate_limit;
  float              _delay_seconds;
  WallpaperScaleMode _scale_mode;

  void cache(cxxopts::ParseResult &args);

public:
  Config(cxxopts::ParseResult &args);
  ~Config() = default;

  void reload(cxxopts::ParseResult &args);
  bool has_changed();

  std::string             get_wallpapers_path();
  bool                    get_shuffle_wallpapers();
  std::deque<std::string> get_wallpapers();
  WallpaperScaleMode      get_scale_mode();
  float                   get_fade_speed();
  unsigned int            get_framerate_limit();
  float                   get_delay_seconds();
  bool                    get_debug();
};
