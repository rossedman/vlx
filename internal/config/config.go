// Package config loads vlx settings with Viper: flags, then VLX_*
// environment variables, then the config file, then defaults.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
)

// Keys, shared by flags, environment variables and the file.
const (
	KeyAllowWrites  = "allow_writes"
	KeyTimeout      = "timeout"
	KeyMaxFrameSize = "max_frame_size"
	KeyMaxReplies   = "max_replies"
	KeySearchPaths  = "search_paths"
	KeyRedact       = "redact"
	KeySafety       = "safety"
	KeyNoColor      = "no_color"
	KeyOutput       = "output"
)

// Config is the resolved configuration.
type Config struct {
	AllowWrites  bool             `mapstructure:"allow_writes" yaml:"allow_writes"`
	Timeout      time.Duration    `mapstructure:"timeout" yaml:"timeout"`
	MaxFrameSize int              `mapstructure:"max_frame_size" yaml:"max_frame_size"`
	MaxReplies   int              `mapstructure:"max_replies" yaml:"max_replies"`
	SearchPaths  []string         `mapstructure:"search_paths" yaml:"search_paths"`
	Redact       []string         `mapstructure:"redact" yaml:"redact"`
	Safety       safety.Overrides `mapstructure:"safety" yaml:"safety"`
	NoColor      bool             `mapstructure:"no_color" yaml:"no_color"`
	Output       string           `mapstructure:"output" yaml:"-"`

	// File is the config file that was loaded, or "".
	File string `mapstructure:"-" yaml:"-"`
}

// SetDefaults registers the default values on v.
func SetDefaults(v *viper.Viper) {
	v.SetDefault(KeyAllowWrites, false)
	v.SetDefault(KeyTimeout, 10*time.Second)
	v.SetDefault(KeyMaxFrameSize, 16<<20)
	v.SetDefault(KeyMaxReplies, 1000)
	v.SetDefault(KeySearchPaths, []string{})
	v.SetDefault(KeyRedact, render.DefaultRedact)
	v.SetDefault(KeySafety+".read", []string{})
	v.SetDefault(KeySafety+".write", []string{})
	v.SetDefault(KeySafety+".danger", []string{})
	v.SetDefault(KeyNoColor, false)
	v.SetDefault(KeyOutput, "text")
}

// DefaultPath is $XDG_CONFIG_HOME/vlx/config.yaml (os.UserConfigDir).
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "vlx", "config.yaml")
}

// Load reads the config file (explicit, or the default path if it exists)
// into v, applies the environment, and decodes the result.
func Load(v *viper.Viper, explicit string) (*Config, error) {
	v.SetEnvPrefix("VLX")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()

	file := explicit
	if file == "" {
		if p := DefaultPath(); p != "" {
			if _, err := os.Stat(p); err == nil {
				file = p
			}
		}
	}
	if file != "" {
		if err := CheckPermissions(file); err != nil {
			return nil, err
		}
		v.SetConfigFile(file)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("config %s: %w", file, err)
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	c.File = file
	if os.Getenv("NO_COLOR") != "" {
		c.NoColor = true
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks value ranges and patterns.
func (c *Config) Validate() error {
	var errs []error
	if c.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("timeout must be positive, got %s", c.Timeout))
	}
	if c.MaxFrameSize < 4096 || c.MaxFrameSize > 1<<30 {
		errs = append(errs, fmt.Errorf("max_frame_size must be between 4096 and 1073741824, got %d", c.MaxFrameSize))
	}
	if c.MaxReplies < 1 {
		errs = append(errs, fmt.Errorf("max_replies must be at least 1, got %d", c.MaxReplies))
	}
	if c.Output != "text" && c.Output != "json" {
		errs = append(errs, fmt.Errorf("output must be text or json, got %q", c.Output))
	}
	if err := c.Safety.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// InsecureError reports a config file that others could modify.
type InsecureError struct {
	Path   string
	Reason string
}

func (e *InsecureError) Error() string {
	return fmt.Sprintf("refusing to load %s: %s (the config can enable writes; fix with 'chmod go-w %s')", e.Path, e.Reason, e.Path)
}

// CheckPermissions refuses config files that are group- or world-writable
// or owned by someone other than the current user or root, because the
// config can turn on allow_writes.
func CheckPermissions(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("config file %s does not exist", path)
		}
		return err
	}
	if !fi.Mode().IsRegular() {
		return &InsecureError{path, "not a regular file"}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return &InsecureError{path, fmt.Sprintf("it is writable by group or others (mode %04o)", fi.Mode().Perm())}
	}
	return checkOwner(path, fi)
}
