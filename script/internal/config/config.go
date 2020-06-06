package config

// Config contains configuration about install, build, test and race.
type Config struct {
	GoRootLatest string `toml:"go_root_latest"`
	GoRoot1108   string `toml:"go_root_1_10_8"`
}
