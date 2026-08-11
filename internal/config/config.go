// Package config owns Concoct's strict project and user configuration model.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultMaxCompleted = 20
	DefaultMaxAge       = 14 * 24 * time.Hour
	DefaultMaxLogBytes  = int64(256 * 1024)
	DefaultMaxTotal     = int64(20 * 1024 * 1024)
)

type profile struct {
	Adapter   string `yaml:"adapter"`
	Model     string `yaml:"model"`
	Reasoning string `yaml:"reasoning"`
	Timeout   string `yaml:"timeout"`
}

type fileConfig struct {
	Git struct {
		AutoPush bool `yaml:"auto-push"`
	} `yaml:"git"`
	Exec struct {
		Adapter   string             `yaml:"adapter"`
		Roles     map[string]profile `yaml:"roles"`
		Retention struct {
			MaxCompleted int    `yaml:"max-completed"`
			MaxAge       string `yaml:"max-age"`
			MaxLogBytes  int64  `yaml:"max-log-bytes"`
			MaxTotal     int64  `yaml:"max-total-bytes"`
		} `yaml:"retention"`
	} `yaml:"exec"`
}

type Overrides struct{ Adapter, Model, Reasoning, Timeout string }
type Defaults struct {
	Adapter, Model, Reasoning string
	Timeout                   time.Duration
	Roles                     map[string]ProfileDefaults
}
type ProfileDefaults struct {
	Model, Reasoning string
	Timeout          time.Duration
}
type Value struct{ Value, Source string }
type Retention struct {
	MaxCompleted int
	MaxAge       time.Duration
	MaxLogBytes  int64
	MaxTotal     int64
}
type Resolved struct {
	Adapter, Model, Reasoning Value
	Timeout                   time.Duration
	TimeoutSource             string
	Retention                 Retention
	RetentionSource           map[string]string
}

func Resolve(root, role string, override Overrides, defaults Defaults) (Resolved, error) {
	projectPath := filepath.Join(root, ".concoct", "config.yaml")
	project, projectExists, err := read(projectPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("parse .concoct/config.yaml: %w", err)
	}
	userPath, err := UserPath()
	if err != nil {
		return Resolved{}, err
	}
	user, userExists, err := read(userPath)
	if err != nil {
		return Resolved{}, fmt.Errorf("parse user configuration %s: %w", userPath, err)
	}
	if err := validateFile(project, projectExists, projectPath); err != nil {
		return Resolved{}, err
	}
	if err := validateFile(user, userExists, userPath); err != nil {
		return Resolved{}, err
	}

	resolved := Resolved{
		Adapter:   Value{defaults.Adapter, "adapter general default"},
		Model:     Value{defaults.Model, "adapter general default"},
		Reasoning: Value{defaults.Reasoning, "adapter general default"},
		Timeout:   defaults.Timeout, TimeoutSource: "adapter general default",
		Retention:       Retention{DefaultMaxCompleted, DefaultMaxAge, DefaultMaxLogBytes, DefaultMaxTotal},
		RetentionSource: map[string]string{"max-completed": "built-in default", "max-age": "built-in default", "max-log-bytes": "built-in default", "max-total-bytes": "built-in default"},
	}
	if roleDefault, ok := defaults.Roles[role]; ok {
		applyDefaults(&resolved, roleDefault, "adapter role default")
	}
	if userExists {
		if user.Exec.Adapter != "" {
			resolved.Adapter = Value{user.Exec.Adapter, "user configuration"}
		}
		applyRole(&resolved, user.Exec.Roles[role], "user role configuration")
		applyRetention(&resolved, user, "user configuration")
	}
	if projectExists {
		if project.Exec.Adapter != "" {
			resolved.Adapter = Value{project.Exec.Adapter, "project configuration"}
		}
		applyRole(&resolved, project.Exec.Roles[role], "project role configuration")
		applyRetention(&resolved, project, "project configuration")
	}
	if override.Adapter != "" {
		resolved.Adapter = Value{override.Adapter, "invocation override"}
	}
	if override.Model != "" {
		resolved.Model = Value{override.Model, "invocation override"}
	}
	if override.Reasoning != "" {
		resolved.Reasoning = Value{override.Reasoning, "invocation override"}
	}
	if override.Timeout != "" {
		d, parseErr := parseDuration("timeout", override.Timeout)
		if parseErr != nil {
			return Resolved{}, parseErr
		}
		resolved.Timeout, resolved.TimeoutSource = d, "invocation override"
	}
	if strings.TrimSpace(resolved.Adapter.Value) == "" {
		return Resolved{}, fmt.Errorf("no execution adapter resolved")
	}
	if resolved.Timeout < time.Second || resolved.Timeout > 24*time.Hour {
		return Resolved{}, fmt.Errorf("timeout must be between 1s and 24h")
	}
	if resolved.Retention.MaxCompleted < 1 || resolved.Retention.MaxAge < time.Hour || resolved.Retention.MaxLogBytes < 1024 || resolved.Retention.MaxTotal < resolved.Retention.MaxLogBytes {
		return Resolved{}, fmt.Errorf("retention requires max-completed >= 1, max-age >= 1h, max-log-bytes >= 1024, and max-total-bytes >= max-log-bytes")
	}
	return resolved, nil
}

func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user configuration directory: %w", err)
	}
	return filepath.Join(dir, "concoct", "config.yaml"), nil
}

func AutoPush(root string) (bool, error) {
	path := filepath.Join(root, ".concoct", "config.yaml")
	c, exists, err := read(path)
	if err != nil {
		return false, fmt.Errorf("parse .concoct/config.yaml: %w", err)
	}
	if err := validateFile(c, exists, path); err != nil {
		return false, err
	}
	return exists && c.Git.AutoPush, nil
}

func read(path string) (fileConfig, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fileConfig{}, false, nil
	}
	if err != nil {
		return fileConfig{}, false, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return fileConfig{}, true, nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c fileConfig
	if err := dec.Decode(&c); err != nil {
		return fileConfig{}, false, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return fileConfig{}, false, fmt.Errorf("multiple YAML documents are not supported")
		}
		return fileConfig{}, false, err
	}
	return c, true, nil
}

func validateFile(c fileConfig, exists bool, source string) error {
	if !exists {
		return nil
	}
	known := map[string]bool{"product-owner": true, "task-planner": true, "developer": true, "reviewer": true, "archivist": true, "integrator": true}
	for role := range c.Exec.Roles {
		if !known[role] {
			return fmt.Errorf("%s: unknown exec role %q", source, role)
		}
		if value := c.Exec.Roles[role].Timeout; value != "" {
			if _, err := parseDuration("timeout", value); err != nil {
				return fmt.Errorf("%s: role %s: %w", source, role, err)
			}
		}
	}
	if value := c.Exec.Retention.MaxAge; value != "" {
		if _, err := parseDuration("retention max-age", value); err != nil {
			return fmt.Errorf("%s: %w", source, err)
		}
	}
	return nil
}

func applyDefaults(r *Resolved, p ProfileDefaults, source string) {
	if p.Model != "" {
		r.Model = Value{p.Model, source}
	}
	if p.Reasoning != "" {
		r.Reasoning = Value{p.Reasoning, source}
	}
	if p.Timeout != 0 {
		r.Timeout, r.TimeoutSource = p.Timeout, source
	}
}
func applyRole(r *Resolved, p profile, source string) {
	if p.Adapter != "" {
		r.Adapter = Value{p.Adapter, source}
	}
	if p.Model != "" {
		r.Model = Value{p.Model, source}
	}
	if p.Reasoning != "" {
		r.Reasoning = Value{p.Reasoning, source}
	}
	if p.Timeout != "" {
		if d, err := parseDuration("timeout", p.Timeout); err == nil {
			r.Timeout, r.TimeoutSource = d, source
		}
	}
}
func applyRetention(r *Resolved, c fileConfig, source string) {
	x := c.Exec.Retention
	if x.MaxCompleted != 0 {
		r.Retention.MaxCompleted, r.RetentionSource["max-completed"] = x.MaxCompleted, source
	}
	if x.MaxAge != "" {
		if d, err := parseDuration("retention max-age", x.MaxAge); err == nil {
			r.Retention.MaxAge, r.RetentionSource["max-age"] = d, source
		}
	}
	if x.MaxLogBytes != 0 {
		r.Retention.MaxLogBytes, r.RetentionSource["max-log-bytes"] = x.MaxLogBytes, source
	}
	if x.MaxTotal != 0 {
		r.Retention.MaxTotal, r.RetentionSource["max-total-bytes"] = x.MaxTotal, source
	}
}
func parseDuration(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return d, nil
}
