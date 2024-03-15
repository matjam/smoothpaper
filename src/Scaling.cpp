#include "Scaling.hpp"

#include <spdlog/spdlog.h>

void scale_horizontal_fit(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite) {
  // scale the wallpaper sprite so that it fits the render window horizontally.
  // the wallpaper sprite will be centered vertically.

  auto render_window_size     = render_window->getSize();
  auto wallpaper_texture_size = wallpaper_sprite->getTexture()->getSize();

  spdlog::debug("render window size: {}x{}", render_window_size.x, render_window_size.y);
  spdlog::debug("wallpaper texture size: {}x{}", wallpaper_texture_size.x, wallpaper_texture_size.y);

  float scale =
      static_cast<float>(static_cast<float>(render_window_size.x)) / static_cast<float>(wallpaper_texture_size.x);
  wallpaper_sprite->setScale(scale, scale);

  float y_offset =
      (static_cast<float>(render_window_size.y) - (static_cast<float>(wallpaper_texture_size.y) * scale)) / 2;
  wallpaper_sprite->setPosition(0, y_offset);

  spdlog::debug("wallpaper sprite scale: {}", scale);
}

void scale_vertical_fit(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite) {
  // scale the wallpaper sprite so that it fits the render window vertically.
  // the wallpaper sprite will be centered horizontally.

  auto render_window_size     = render_window->getSize();
  auto wallpaper_texture_size = wallpaper_sprite->getTexture()->getSize();

  spdlog::debug("render window size: {}x{}", render_window_size.x, render_window_size.y);
  spdlog::debug("wallpaper texture size: {}x{}", wallpaper_texture_size.x, wallpaper_texture_size.y);

  float scale =
      static_cast<float>(static_cast<float>(render_window_size.y)) / static_cast<float>(wallpaper_texture_size.y);
  wallpaper_sprite->setScale(scale, scale);

  float x_offset =
      (static_cast<float>(render_window_size.x) - (static_cast<float>(wallpaper_texture_size.x) * scale)) / 2;
  wallpaper_sprite->setPosition(x_offset, 0);

  spdlog::debug("wallpaper sprite scale: {}", scale);
}

void scale_stretched(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite) {
  // scale the wallpaper sprite so that it fits the render window both horizontally and
  // vertically. The wallpaper sprite will be centered.

  auto render_window_size     = render_window->getSize();
  auto wallpaper_texture_size = wallpaper_sprite->getTexture()->getSize();

  spdlog::debug("render window size: {}x{}", render_window_size.x, render_window_size.y);
  spdlog::debug("wallpaper texture size: {}x{}", wallpaper_texture_size.x, wallpaper_texture_size.y);

  float x_scale =
      static_cast<float>(static_cast<float>(render_window_size.x)) / static_cast<float>(wallpaper_texture_size.x);
  float y_scale =
      static_cast<float>(static_cast<float>(render_window_size.y)) / static_cast<float>(wallpaper_texture_size.y);
  wallpaper_sprite->setScale(x_scale, y_scale);

  spdlog::debug("wallpaper sprite scale: {}x{}", x_scale, y_scale);

  wallpaper_sprite->setPosition(0, 0);
}

void scale(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite, WallpaperScaleMode mode) {
  switch (mode) {
  case WallpaperScaleMode::HorizontalFit:
    scale_horizontal_fit(render_window, wallpaper_sprite);
    break;
  case WallpaperScaleMode::VerticalFit:
    scale_vertical_fit(render_window, wallpaper_sprite);
    break;
  case WallpaperScaleMode::StretchedFit:
    scale_stretched(render_window, wallpaper_sprite);
    break;
  }
}
