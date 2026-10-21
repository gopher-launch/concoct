package selection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gopher-launch/concoct/internal/activework"
	"github.com/gopher-launch/concoct/internal/codexapp"
	"github.com/gopher-launch/concoct/internal/roadmap"
)

type hostSession struct {
	mu                           sync.Mutex
	owner                        *activework.Owner
	store                        *activework.Store
	project                      projectSnapshot
	workspace                    string
	parent                       parentBinding
	existingBranches             map[string]bool
	threadID, taskID, taskBranch string
	claimedArgs                  claimArguments
	checkpointSaved              bool
}

type claimArguments struct {
	TaskID, TaskBranch, SelectionRationale string
}

func (s *hostSession) handle(_ context.Context, call codexapp.ToolCall) (codexapp.PreparedToolResponse, error) {
	if call.Role != "reagent" || call.ThreadID != s.threadID || call.TurnID == "" || call.CallID == "" || call.Namespace != "" {
		return codexapp.PreparedToolResponse{}, fmt.Errorf("callback does not belong to the reagent turn")
	}
	switch call.Tool {
	case "claim_task":
		var args struct {
			TaskID             string `json:"task_id"`
			TaskBranch         string `json:"task_branch"`
			SelectionRationale string `json:"selection_rationale"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return codexapp.PreparedToolResponse{}, err
		}
		claim := claimArguments{args.TaskID, args.TaskBranch, args.SelectionRationale}
		return codexapp.PreparedToolResponse{Success: true, Text: "Claim acknowledged; continue engineering this task.", Commit: func(context.Context) error {
			return s.claim(call.CallID, claim)
		}}, nil
	case "save_checkpoint":
		var args struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return codexapp.PreparedToolResponse{}, err
		}
		return codexapp.PreparedToolResponse{Success: true, Text: "Checkpoint saved.", Commit: func(context.Context) error {
			return s.checkpoint(call.CallID, call.TurnID, args.Content)
		}}, nil
	default:
		return codexapp.PreparedToolResponse{}, fmt.Errorf("unsupported selection tool %q", call.Tool)
	}
}

func (s *hostSession) claim(callID string, args claimArguments) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, name, err := callReceipt("claim", callID, args)
	if err != nil {
		return err
	}
	exists, err := s.owner.CheckReceipt(name, receipt)
	if err != nil {
		return err
	}
	if s.taskID != "" {
		if s.claimedArgs != args {
			return fmt.Errorf("second or changed task claim is forbidden")
		}
		if exists {
			return nil
		}
		return s.owner.RecordReceipt(name, receipt)
	}
	entry, err := eligibleEntry(s.project.Document, args.TaskID)
	if err != nil {
		return err
	}
	objective, acceptance, err := taskScope(entry)
	if err != nil {
		return err
	}
	if s.existingBranches[args.TaskBranch] {
		return fmt.Errorf("task branch %q existed before selection", args.TaskBranch)
	}
	if err := verifyClaimWorkspace(s.store, s.workspace, s.parent, args.TaskBranch); err != nil {
		return err
	}
	if err := s.owner.ClaimOrConfirm(activework.StartSpec{TaskID: args.TaskID, ParentBranch: s.parent.Branch, ParentCommit: s.parent.Commit, TaskBranch: args.TaskBranch, Objective: objective, Acceptance: acceptance, SelectionRationale: args.SelectionRationale}); err != nil {
		return err
	}
	if !exists {
		if err := s.owner.RecordReceipt(name, receipt); err != nil {
			return err
		}
	}
	s.taskID, s.taskBranch, s.claimedArgs = args.TaskID, args.TaskBranch, args
	return nil
}

func (s *hostSession) checkpoint(callID, turnID, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.taskID == "" {
		return fmt.Errorf("checkpoint requires an acknowledged task claim")
	}
	receipt, name, err := callReceipt("checkpoint", callID, struct{ CallID, TurnID, Content string }{callID, turnID, content})
	if err != nil {
		return err
	}
	exists, err := s.owner.CheckReceipt(name, receipt)
	if err != nil || exists {
		if exists {
			s.checkpointSaved = true
		}
		return err
	}
	if err := s.owner.SaveCheckpoint(content); err != nil {
		return err
	}
	if err := s.owner.RecordReceipt(name, receipt); err != nil {
		return err
	}
	s.checkpointSaved = true
	return nil
}

func callReceipt(kind, callID string, args any) ([]byte, string, error) {
	if len(callID) > 256 {
		return nil, "", fmt.Errorf("callback ID is too long")
	}
	data, err := json.Marshal(args)
	if err != nil {
		return nil, "", err
	}
	hash := sha256.Sum256([]byte(callID))
	return data, kind + "-" + hex.EncodeToString(hash[:12]), nil
}

func eligibleEntry(doc *roadmap.Document, id string) (roadmap.Entry, error) {
	for _, entry := range doc.EligibleTasks() {
		if entry.ID == id {
			return entry, nil
		}
	}
	return roadmap.Entry{}, fmt.Errorf("task %q is not ready with satisfied dependencies", id)
}

func taskScope(entry roadmap.Entry) (string, string, error) {
	objectiveStart := strings.Index(entry.Body, "**Objective:**")
	acceptanceStart := strings.Index(entry.Body, "**Acceptance criteria:**")
	if objectiveStart < 0 || acceptanceStart <= objectiveStart {
		return "", "", fmt.Errorf("task %s lacks objective or acceptance criteria", entry.ID)
	}
	objective := strings.TrimSpace(entry.Body[objectiveStart+len("**Objective:**") : acceptanceStart])
	acceptance := entry.Body[acceptanceStart+len("**Acceptance criteria:**"):]
	if end := strings.Index(acceptance, "**Verification:**"); end >= 0 {
		acceptance = acceptance[:end]
	}
	acceptance = strings.TrimSpace(acceptance)
	if objective == "" || acceptance == "" {
		return "", "", fmt.Errorf("task %s has empty scope", entry.ID)
	}
	return objective, acceptance, nil
}
