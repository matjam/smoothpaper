cmake_minimum_required(VERSION 3.28)
set(CMAKE_VERBOSE_MAKEFILE ON)

file(STRINGS "version.txt" version REGEX "^([0-9]+)\\.([0-9]+)\\.([0-9]+)")

project(smoothpaper
    VERSION ${version}
    DESCRIPTION "A wallpaper manager with smooth transitions for X11 Window Managers"
    LANGUAGES CXX
)

configure_file(src/Version.hpp.in src/Version.hpp)


string(TOLOWER ${CMAKE_BUILD_TYPE} build_type)
if (build_type STREQUAL "debug")
    add_compile_options(-O0 -g -Wall -Wextra -Wpedantic -Werror)
else()
    add_compile_options(-O2 -Wall -Wextra -Wpedantic -Werror)
endif()


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
