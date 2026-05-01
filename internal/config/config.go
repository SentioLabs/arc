// Package config provides the single source of truth for arc CLI and server settings.
// Settings are stored in TOML format at ~/.arc/config.toml.
package config

// Config is the full arc configuration document.
type Config struct {
	CLI     CLIConfig     `toml:"cli"     json:"cli"`
	Server  ServerConfig  `toml:"server"  json:"server"`
	Share   ShareConfig   `toml:"share"   json:"share"`
	Updates UpdatesConfig `toml:"updates" json:"updates"`
}

// CLIConfig holds settings the arc CLI uses to reach the server.
type CLIConfig struct {
	Server string `toml:"server" json:"server"`
}

// ServerConfig holds settings the arc server uses for its own runtime.
type ServerConfig struct {
	Port   int    `toml:"port"    json:"port"`
	DBPath string `toml:"db_path" json:"db_path"`
}

// ShareConfig holds defaults for `arc share` and the web share UI.
type ShareConfig struct {
	Author string `toml:"author" json:"author"`
	Server string `toml:"server" json:"server"`
}

// UpdatesConfig holds update-channel settings for `arc self`.
type UpdatesConfig struct {
	Channel string `toml:"channel" json:"channel"`
}

// Default returns a Config populated with built-in defaults.
func Default() *Config {
	return &Config{
		CLI:     CLIConfig{Server: "http://localhost:7432"},
		Server:  ServerConfig{Port: 7432, DBPath: "~/.arc/data.db"},
		Share:   ShareConfig{Server: "https://arcplanner.sentiolabs.io"},
		Updates: UpdatesConfig{Channel: "stable"},
	}
}

// RequiresRestart returns dotted keys whose changes don't take effect in a
// running arc-server until the server restarts. Used by the API + web UI to
// surface a "requires restart" warning.
func RequiresRestart() []string {
	return []string{"server.port", "server.db_path"}
}
