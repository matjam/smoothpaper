cmake_minimum_required(VERSION 3.28)
set(CMAKE_VERBOSE_MAKEFILE ON)

project(smoothpaper
    VERSION 0.1.0
    DESCRIPTION "A wallpaper manager with smooth transitions for X11 Window Managers"
    LANGUAGES CXX
)

configure_file(src/Version.hpp.in src/Version.hpp)

add_compile_options(-O3 -Wall -Wextra -Wpedantic -Werror)

# ---- libraries ----
find_package(fmt CONFIG REQUIRED)
find_package(X11 REQUIRED)
find_package(SFML COMPONENTS system window graphics CONFIG REQUIRED)
find_package(spdlog CONFIG REQUIRED)
find_package(toml11 CONFIG REQUIRED)

# ---- target ----
add_executable(
    smoothpaper
    src/main.cpp
    src/Config.cpp
    src/Config.hpp
    src/Window.cpp
    src/Window.hpp
    src/Scaling.cpp
    src/Scaling.hpp
)

target_compile_features(smoothpaper PRIVATE cxx_std_20)
target_link_libraries(smoothpaper PRIVATE 
    fmt::fmt 
    X11::X11 
    sfml-system sfml-network sfml-graphics sfml-window 
    spdlog::spdlog 
    toml11::toml11
)
target_include_directories(smoothpaper PUBLIC "${PROJECT_BINARY_DIR}/src")