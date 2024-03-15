#pragma once

#include <SFML/Graphics.hpp>

#include "Config.hpp"

void scale_horizontal_fit(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite);

void scale_vertical_fit(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite);

void scale_stretched(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite);

void scale(sf::RenderWindow *render_window, sf::Sprite *wallpaper_sprite, WallpaperScaleMode scale_mode);