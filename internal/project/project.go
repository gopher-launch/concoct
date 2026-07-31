package project

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	concoct "github.com/gopher-launch/concoct"
	"github.com/gopher-launch/concoct/internal/workflow"
)

func Discover(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	for {
		if regular(filepath.Join(dir, "AGENTS.md")) && regular(filepath.Join(dir, ".concoct", "roadmap.md")) && regular(filepath.Join(dir, ".concoct", "capabilities.md")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("uninitialized: no Concoct project found from %s; run concoct init <project>", start)
}

func Initialize(base, target string, out io.Writer) (err error) {
	target = strings.TrimSpace(target)
	if target == "" || target == "." || target == ".." {
		return fmt.Errorf("unsafe project target %q; no files were created", target)
	}
	if filepath.IsAbs(target) {
		target = filepath.Clean(target)
	} else {
		target = filepath.Join(base, target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target: %w; no files were created", err)
	}
	parent := filepath.Dir(target)
	if filepath.Clean(target) == filepath.Clean(parent) {
		return fmt.Errorf("unsafe project target %s; no files were created", target)
	}
	if _, statErr := os.Lstat(target); statErr == nil {
		return fmt.Errorf("target already exists: %s; it was not modified", target)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("inspect target %s: %w; it was not modified", target, statErr)
	}
	if info, statErr := os.Stat(parent); statErr != nil || !info.IsDir() {
		return fmt.Errorf("destination parent must be an existing directory: %s; no files were created", parent)
	}
	created := false
	defer func() {
		if err != nil && created {
			err = fmt.Errorf("%w; partial target remains at %s; inspect it, then remove that exact directory before retrying", err, target)
		}
	}()
	if err = os.Mkdir(target, 0o755); err != nil {
		return fmt.Errorf("create target: %w", err)
	}
	created = true
	if err = copyTemplates(target, filepath.Base(target)); err != nil {
		return fmt.Errorf("copy templates: %w", err)
	}
	if err = os.MkdirAll(filepath.Join(target, ".concoct", "archive"), 0o755); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}
	if err = writeBootstrap(target); err != nil {
		return fmt.Errorf("write bootstrap guidance: %w", err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = target
		if output, runErr := cmd.CombinedOutput(); runErr != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], runErr, strings.TrimSpace(string(output)))
		}
	}
	if _, err = Discover(target); err != nil {
		return fmt.Errorf("validate initialized project: %w", err)
	}
	report := workflow.Detect(target)
	if report.State != workflow.Ready {
		return fmt.Errorf("validate initialized project: expected ready, got %s (%s)", report.State, strings.Join(report.Diagnostics, "; "))
	}
	fmt.Fprintf(out, "Concoct project initialized at %s\nState: ready\nGenerated files: staged (no commit created)\nNext: concoct next\nInspect: concoct status\n", target)
	return nil
}

func copyTemplates(target, projectName string) error {
	return fs.WalkDir(concoct.Templates, "templates", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "templates" {
			return nil
		}
		rel, err := filepath.Rel("templates", path)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(concoct.Templates, path)
		if err != nil {
			return err
		}
		data = []byte(strings.ReplaceAll(string(data), "<project-name>", projectName))
		return os.WriteFile(dst, data, 0o644)
	})
}

func writeBootstrap(target string) error {
	content := "# Concoct bootstrap\n\nThis project is in the `ready` state.\n\nRun the read-only Product Owner recommendation step before selecting work or performing roadmap intake.\n\nRecommended next command: `concoct next`\n"
	return os.WriteFile(filepath.Join(target, ".concoct", "current", "bootstrap-prompt.md"), []byte(content), 0o644)
}

func regular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
