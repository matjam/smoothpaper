/*
   Copyright 2024 Nathan Ollerenshaw

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

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
