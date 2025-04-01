package smoothpaper

import _ "embed"

//go:embed version.txt
var Version string

//go:embed smoothpaper.toml
var DefaultConfig string
