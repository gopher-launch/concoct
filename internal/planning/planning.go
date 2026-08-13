// Package planning owns the shared pre-launch planning boundary.
package planning

import (
	"fmt"
	"path/filepath"

	"github.com/gopher-launch/concoct/internal/gitrepo"
	"github.com/gopher-launch/concoct/internal/prompt"
	"github.com/gopher-launch/concoct/internal/workflow"
)

// Session records a planning prompt's exact Git context. Rollback is valid
// only before an agent process may have started.
type Session struct {
	Root    string
	ItemID  string
	Request prompt.Request
	repo    *gitrepo.Repository
	start   gitrepo.TaskStart
}

// Complete validates and commits a supervised planner candidate. The planner
// may author only the two active artifacts and the selected roadmap status;
// Git metadata remains owned by the outer executable.
func (s *Session) Complete() (workflow.TransitionResult, error) {
	if report := workflow.Detect(s.Root); report.State != workflow.Planned || report.RoadmapItem != s.ItemID {
		return workflow.TransitionResult{}, fmt.Errorf("planner output is not a valid planned transition for %s", s.ItemID)
	}
	if s.repo == nil {
		return workflow.TransitionResult{Message: "Task Planner transition completed"}, nil
	}
	head, err := s.repo.Head()
	if err != nil || head != s.start.Base {
		return workflow.TransitionResult{}, fmt.Errorf("Task Planner may not create Git commits; expected unchanged planning base %s", s.start.Base)
	}
	branch, err := s.repo.Branch()
	if err != nil || branch != s.start.Branch {
		return workflow.TransitionResult{}, fmt.Errorf("planning checkout drift: expected task branch %s", s.start.Branch)
	}
	entries, err := s.repo.StatusEntries()
	if err != nil {
		return workflow.TransitionResult{}, err
	}
	allowed := map[string]bool{".concoct/current/task-plan.md": true, ".concoct/current/notes.md": true, ".concoct/roadmap.md": true}
	seen := map[string]bool{}
	for _, entry := range entries {
		for _, path := range entry.Paths {
			path = filepath.ToSlash(path)
			if !allowed[path] {
				return workflow.TransitionResult{}, fmt.Errorf("Task Planner transition contains forbidden path %s", path)
			}
			seen[path] = true
		}
	}
	if !seen[".concoct/current/task-plan.md"] || !seen[".concoct/current/notes.md"] || !seen[".concoct/roadmap.md"] {
		return workflow.TransitionResult{}, fmt.Errorf("Task Planner transition requires task-plan.md, notes.md, and selected roadmap status changes")
	}
	if err := s.repo.AddAll(); err != nil {
		return workflow.TransitionResult{}, err
	}
	if err := s.repo.Commit("concoct: plan " + s.ItemID); err != nil {
		return workflow.TransitionResult{}, err
	}
	commit, err := s.repo.Head()
	if err != nil {
		return workflow.TransitionResult{}, err
	}
	if err := s.repo.Clean(); err != nil {
		return workflow.TransitionResult{}, err
	}
	return workflow.TransitionResult{Message: "Task Planner transition completed", Committed: true, Commit: commit}, nil
}

func Start(root, itemID string) (*Session, error) {
	if err := workflow.ValidatePlanItem(root, itemID); err != nil {
		return nil, err
	}
	s := &Session{Root: root, ItemID: itemID, Request: prompt.Request{Command: "plan", RoadmapID: itemID}}
	repo, ok, err := gitrepo.Open(root)
	if err != nil {
		return nil, err
	}
	if !ok {
		return s, nil
	}
	title, err := workflow.PlanItemTitle(root, itemID)
	if err != nil {
		return nil, err
	}
	start, err := repo.CreateTaskBranch(itemID, title)
	if err != nil {
		return nil, err
	}
	s.repo, s.start = repo, start
	s.Request.GitTrunk, s.Request.GitTaskBranch, s.Request.GitBase = start.Trunk, start.Branch, start.Base
	return s, nil
}

func (s *Session) Render() ([]byte, error) {
	content, err := prompt.Render(s.Root, s.Request)
	if err != nil {
		_ = s.Rollback()
	}
	return content, err
}

func (s *Session) Rollback() error {
	if s == nil || s.repo == nil {
		return nil
	}
	if err := s.repo.Checkout(s.start.Trunk); err != nil {
		return fmt.Errorf("restore planning trunk: %w", err)
	}
	if err := s.repo.DeleteBranch(s.start.Branch); err != nil {
		return fmt.Errorf("remove unused planning branch: %w", err)
	}
	s.repo = nil
	return nil
}
