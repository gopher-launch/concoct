// Package config owns Concoct's strict project and user configuration model.
package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	HardRunMaxActions   = 20
	HardRunMaxCycles    = 3
)

type profile struct {
	Adapter   string       `yaml:"adapter"`
	Model     string       `yaml:"model"`
	Reasoning string       `yaml:"reasoning"`
	Timeout   string       `yaml:"timeout"`
	Budget    budgetConfig `yaml:"budget"`
}

type budgetConfig struct {
	WarnElapsed       string `yaml:"warn-elapsed"`
	HardElapsed       string `yaml:"hard-elapsed"`
	WarnActivity      int64  `yaml:"warn-activity"`
	HardActivity      int64  `yaml:"hard-activity"`
	WarnCommandOutput int64  `yaml:"warn-command-output-bytes"`
	HardCommandOutput int64  `yaml:"hard-command-output-bytes"`
	WarnInputTokens   int64  `yaml:"warn-input-tokens"`
	WarnOutputTokens  int64  `yaml:"warn-output-tokens"`
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
			RawEvents    *bool  `yaml:"raw-events"`
		} `yaml:"retention"`
	} `yaml:"exec"`
	Run struct {
		Gates      []string `yaml:"gates"`
		MaxActions *int     `yaml:"max-actions"`
		MaxCycles  *int     `yaml:"max-cycles"`
	} `yaml:"run"`
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
	RawEvents    bool
}
type Resolved struct {
	Adapter, Model, Reasoning Value
	Timeout                   time.Duration
	TimeoutSource             string
	Retention                 Retention
	RetentionSource           map[string]string
	Budget                    Budget
}

type BudgetValue struct {
	Value  int64
	Source string
}
type Budget struct {
	WarnElapsed, HardElapsed             time.Duration
	WarnElapsedSource, HardElapsedSource string
	WarnActivity, HardActivity           BudgetValue
	WarnCommandOutput, HardCommandOutput BudgetValue
	WarnInputTokens, WarnOutputTokens    BudgetValue
}

// RunOverrides are invocation-only restrictions. They can add approval gates
// and lower bounds, but never remove or raise project requirements.
type RunOverrides struct {
	Gates      []string
	MaxActions int
	MaxCycles  int
}

// RunPolicy is the finite effective policy for one coordinator invocation.
type RunPolicy struct {
	Gates      map[string]bool
	MaxActions int
	MaxCycles  int
}

var supportedRunGates = map[string]bool{
	"development": true, "review": true, "archive": true,
}

func (p RunPolicy) Requires(gate string) bool { return p.Gates[gate] }

// ResolveRun composes built-in, user, project, and invocation restrictions.
// Every layer is monotonic: gates are unioned and bounds take the minimum.
func ResolveRun(root string, override RunOverrides) (RunPolicy, error) {
	projectPath := filepath.Join(root, ".concoct", "config.yaml")
	project, projectExists, err := read(projectPath)
	if err != nil {
		return RunPolicy{}, fmt.Errorf("parse .concoct/config.yaml: %w", err)
	}
	userPath, err := UserPath()
	if err != nil {
		return RunPolicy{}, err
	}
	user, userExists, err := read(userPath)
	if err != nil {
		return RunPolicy{}, fmt.Errorf("parse user configuration %s: %w", userPath, err)
	}
	if err := validateFile(project, projectExists, projectPath); err != nil {
		return RunPolicy{}, err
	}
	if err := validateFile(user, userExists, userPath); err != nil {
		return RunPolicy{}, err
	}
	policy := RunPolicy{Gates: map[string]bool{"plan": true, "integration": true}, MaxActions: HardRunMaxActions, MaxCycles: HardRunMaxCycles}
	applyRun := func(c fileConfig) {
		for _, gate := range c.Run.Gates {
			policy.Gates[gate] = true
		}
		if c.Run.MaxActions != nil && *c.Run.MaxActions < policy.MaxActions {
			policy.MaxActions = *c.Run.MaxActions
		}
		if c.Run.MaxCycles != nil && *c.Run.MaxCycles < policy.MaxCycles {
			policy.MaxCycles = *c.Run.MaxCycles
		}
	}
	if userExists {
		applyRun(user)
	}
	if projectExists {
		applyRun(project)
	}
	if err := validateRunOverrides(override, "invocation"); err != nil {
		return RunPolicy{}, err
	}
	for _, gate := range override.Gates {
		policy.Gates[gate] = true
	}
	if override.MaxActions > 0 {
		if override.MaxActions > policy.MaxActions {
			return RunPolicy{}, fmt.Errorf("invocation max-actions %d cannot raise effective bound %d", override.MaxActions, policy.MaxActions)
		}
		policy.MaxActions = override.MaxActions
	}
	if override.MaxCycles > 0 {
		if override.MaxCycles > policy.MaxCycles {
			return RunPolicy{}, fmt.Errorf("invocation max-cycles %d cannot raise effective bound %d", override.MaxCycles, policy.MaxCycles)
		}
		policy.MaxCycles = override.MaxCycles
	}
	return policy, nil
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
		Retention:       Retention{MaxCompleted: DefaultMaxCompleted, MaxAge: DefaultMaxAge, MaxLogBytes: DefaultMaxLogBytes, MaxTotal: DefaultMaxTotal, RawEvents: false},
		RetentionSource: map[string]string{"max-completed": "built-in default", "max-age": "built-in default", "max-log-bytes": "built-in default", "max-total-bytes": "built-in default", "raw-events": "built-in default (disabled)"},
	}
	if roleDefault, ok := defaults.Roles[role]; ok {
		applyDefaults(&resolved, roleDefault, "adapter role default")
	}
	applyBuiltInRoleBudget(&resolved.Budget, role)
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
	if resolved.Budget.HardElapsed > 0 && resolved.Budget.HardElapsed >= resolved.Timeout {
		return Resolved{}, fmt.Errorf("hard-elapsed budget must be lower than timeout")
	}
	if err := validateResolvedBudget(resolved.Budget); err != nil {
		return Resolved{}, err
	}
	return resolved, nil
}

func applyBuiltInRoleBudget(b *Budget, role string) {
	// Warning-only defaults are deliberately above the retained CON-037
	// production observations. They surface amplification without terminating
	// an invocation. Hard bounds remain explicit project/user choices because
	// terminal token totals are not live-enforceable Codex evidence.
	values := map[string]struct {
		elapsed                            time.Duration
		activity, output, input, generated int64
	}{
		"developer": {25 * time.Minute, 120, 4 * 1024 * 1024, 2_000_000, 25_000},
		"reviewer":  {20 * time.Minute, 80, 2 * 1024 * 1024, 750_000, 15_000},
		"archivist": {15 * time.Minute, 50, 1024 * 1024, 500_000, 12_000},
	}
	v, ok := values[role]
	if !ok {
		return
	}
	b.WarnElapsed, b.WarnElapsedSource = v.elapsed, "built-in retained-evidence default"
	b.WarnActivity = BudgetValue{v.activity, "built-in retained-evidence default"}
	b.WarnCommandOutput = BudgetValue{v.output, "built-in retained-evidence default"}
	b.WarnInputTokens = BudgetValue{v.input, "built-in retained-evidence default"}
	b.WarnOutputTokens = BudgetValue{v.generated, "built-in retained-evidence default"}
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

// EvidenceDigest binds pending approvals to project and user configuration
// bytes without retaining configuration content in runtime records.
func EvidenceDigest(root string) (string, error) {
	userPath, err := UserPath()
	if err != nil {
		return "", err
	}
	var material []byte
	for _, item := range []struct{ label, path string }{{"project", filepath.Join(root, ".concoct", "config.yaml")}, {"user", userPath}} {
		path := item.path
		data, readErr := os.ReadFile(path)
		if os.IsNotExist(readErr) {
			material = append(material, []byte(item.label+":absent\n")...)
			continue
		}
		if readErr != nil {
			return "", readErr
		}
		sum := sha256.Sum256(data)
		material = append(material, []byte(item.label+":"+hex.EncodeToString(sum[:])+"\n")...)
	}
	sum := sha256.Sum256(material)
	return hex.EncodeToString(sum[:]), nil
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
		if err := validateBudget(c.Exec.Roles[role].Budget); err != nil {
			return fmt.Errorf("%s: role %s: %w", source, role, err)
		}
	}
	if value := c.Exec.Retention.MaxAge; value != "" {
		if _, err := parseDuration("retention max-age", value); err != nil {
			return fmt.Errorf("%s: %w", source, err)
		}
	}
	if err := validateRunValues(c.Run.Gates, c.Run.MaxActions, c.Run.MaxCycles, source); err != nil {
		return err
	}
	return nil
}

func validateRunValues(gates []string, actions, cycles *int, source string) error {
	seen := map[string]bool{}
	for _, gate := range gates {
		if !supportedRunGates[gate] {
			return fmt.Errorf("%s: unknown run gate %q", source, gate)
		}
		if seen[gate] {
			return fmt.Errorf("%s: duplicate run gate %q", source, gate)
		}
		seen[gate] = true
	}
	if actions != nil && (*actions < 1 || *actions > HardRunMaxActions) {
		return fmt.Errorf("%s: run max-actions must be between 1 and %d when set", source, HardRunMaxActions)
	}
	if cycles != nil && (*cycles < 1 || *cycles > HardRunMaxCycles) {
		return fmt.Errorf("%s: run max-cycles must be between 1 and %d when set", source, HardRunMaxCycles)
	}
	return nil
}

func validateRunOverrides(override RunOverrides, source string) error {
	var actions, cycles *int
	if override.MaxActions != 0 {
		actions = &override.MaxActions
	}
	if override.MaxCycles != 0 {
		cycles = &override.MaxCycles
	}
	return validateRunValues(override.Gates, actions, cycles, source)
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
	applyBudget(&r.Budget, p.Budget, source)
}

func validateBudget(b budgetConfig) error {
	for name, value := range map[string]string{"warn-elapsed": b.WarnElapsed, "hard-elapsed": b.HardElapsed} {
		if value != "" {
			d, err := parseDuration(name, value)
			if err != nil || d <= 0 {
				if err != nil {
					return err
				}
				return fmt.Errorf("%s must be positive", name)
			}
		}
	}
	for name, value := range map[string]int64{"warn-activity": b.WarnActivity, "hard-activity": b.HardActivity, "warn-command-output-bytes": b.WarnCommandOutput, "hard-command-output-bytes": b.HardCommandOutput, "warn-input-tokens": b.WarnInputTokens, "warn-output-tokens": b.WarnOutputTokens} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
	}
	return nil
}

func applyBudget(dst *Budget, src budgetConfig, source string) {
	if src.WarnElapsed != "" {
		dst.WarnElapsed, _ = time.ParseDuration(src.WarnElapsed)
		dst.WarnElapsedSource = source
	}
	if src.HardElapsed != "" {
		dst.HardElapsed, _ = time.ParseDuration(src.HardElapsed)
		dst.HardElapsedSource = source
	}
	for _, item := range []struct {
		value  int64
		target *BudgetValue
	}{{src.WarnActivity, &dst.WarnActivity}, {src.HardActivity, &dst.HardActivity}, {src.WarnCommandOutput, &dst.WarnCommandOutput}, {src.HardCommandOutput, &dst.HardCommandOutput}, {src.WarnInputTokens, &dst.WarnInputTokens}, {src.WarnOutputTokens, &dst.WarnOutputTokens}} {
		if item.value > 0 {
			*item.target = BudgetValue{Value: item.value, Source: source}
		}
	}
}

func validateResolvedBudget(b Budget) error {
	if b.WarnElapsed > 0 && b.HardElapsed > 0 && b.WarnElapsed > b.HardElapsed {
		return fmt.Errorf("warn-elapsed budget cannot exceed hard-elapsed")
	}
	for name, pair := range map[string][2]int64{"activity": {b.WarnActivity.Value, b.HardActivity.Value}, "command-output-bytes": {b.WarnCommandOutput.Value, b.HardCommandOutput.Value}} {
		if pair[0] > 0 && pair[1] > 0 && pair[0] > pair[1] {
			return fmt.Errorf("warning %s budget cannot exceed hard budget", name)
		}
	}
	return nil
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
	if x.RawEvents != nil {
		r.Retention.RawEvents, r.RetentionSource["raw-events"] = *x.RawEvents, source
	}
}
func parseDuration(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return d, nil
}
