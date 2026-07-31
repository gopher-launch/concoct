package workflow

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gopher-launch/concoct/internal/gitrepo"
	"gopkg.in/yaml.v3"
)

type State string

const (
	Ready            State = "ready"
	Planned          State = "planned"
	InProgress       State = "implementation-in-progress"
	Complete         State = "implementation-complete"
	ChangesRequested State = "review-changes-requested"
	Approved         State = "review-approved"
	Blocked          State = "review-blocked"
	Archived         State = "archived"
	Integrating      State = "integrating"
	Integrated       State = "integrated"
	Invalid          State = "invalid"
)

type Report struct {
	Project, RoadmapItem, TaskStatus, LatestReview, ReviewOutcome, CapabilityImpact, GitTrunk, GitTaskBranch, GitArchiveCommit, Next string
	State                                                                                                                            State
	Diagnostics                                                                                                                      []string
	OperationalError                                                                                                                 error
}

func (r Report) String() string {
	var b strings.Builder
	field := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	field("Project", r.Project)
	field("Active roadmap item", r.RoadmapItem)
	field("Phase", string(r.State))
	field("Task status", r.TaskStatus)
	field("Latest review", r.LatestReview)
	field("Review outcome", r.ReviewOutcome)
	field("Capability impact", r.CapabilityImpact)
	field("Git trunk", r.GitTrunk)
	field("Git task branch", r.GitTaskBranch)
	field("Git archive commit", r.GitArchiveCommit)
	for _, d := range r.Diagnostics {
		fmt.Fprintf(&b, "Diagnostic: %s\n", d)
	}
	field("Next", r.Next)
	return b.String()
}

type taskMeta struct {
	ID         string     `yaml:"id"`
	Title      string     `yaml:"title"`
	RoadmapID  string     `yaml:"roadmap-id"`
	Status     string     `yaml:"status"`
	Created    string     `yaml:"created"`
	Updated    string     `yaml:"updated"`
	Remediates string     `yaml:"remediates-review"`
	Impact     impact     `yaml:"capability-impact"`
	Resolution resolution `yaml:"blocked-review-resolution"`
	Git        gitMeta    `yaml:"git"`
}
type gitMeta struct {
	Enabled            bool   `yaml:"enabled"`
	Trunk              string `yaml:"trunk"`
	TaskBranch         string `yaml:"task-branch"`
	Base               string `yaml:"base"`
	ArchiveCommit      string `yaml:"archive-commit"`
	PreIntegrationHead string `yaml:"pre-integration-head"`
	IntegrationCommit  string `yaml:"integration-commit"`
	Status             string `yaml:"status"`
}
type impact struct {
	Type      string   `yaml:"type"`
	IDs       []string `yaml:"ids"`
	Rationale string   `yaml:"rationale"`
}
type resolution struct {
	Review     string   `yaml:"review"`
	Route      string   `yaml:"route"`
	RecordedBy string   `yaml:"recorded-by"`
	Evidence   []string `yaml:"evidence"`
}
type reviewMeta struct {
	TaskID  string `yaml:"task-id"`
	Review  int    `yaml:"review"`
	Status  string `yaml:"status"`
	Created string `yaml:"created"`
	Persona string `yaml:"persona"`
}
type roadItem struct {
	Title, Status, Priority, Archive string
	Dependencies                     []string
	Prerequisites                    []string
}

type capabilityRecord struct {
	Status      string
	Limitations string
	Archives    []string
}

// PlanEligibility is the validated, deterministic context for one roadmap
// item entering Task Planner work.
type PlanEligibility struct {
	Prerequisites []PlanPrerequisite
}

type PlanPrerequisite struct {
	ID, Status, Limitations string
	Archives                []string
}

// NextActionEvidence is deterministic, validated repository evidence for
// Product Owner judgment. Eligibility is structural, never a priority choice.
type NextActionEvidence struct {
	RoadmapItems     []NextRoadmapItem
	Capabilities     []NextCapability
	SupportedOrigins []string
}

type NextRoadmapItem struct {
	ID, Title, Status, Priority, Archive, Blocker string
	Dependencies, Prerequisites                   []string
	Eligible                                      bool
}

type NextCapability struct {
	ID, Status, Limitations string
	Archives                []string
}

// PromptContext exposes validated, read-only workflow evidence needed by role
// prompt rendering without making the parser's internal metadata public.
type PromptContext struct {
	Report           Report
	TaskTitle        string
	RemediatesReview string
	ResolutionRoute  string
	ReviewFiles      []string
	NextReview       string
}

type GitContext struct {
	Enabled, Archived                                 bool
	ID, Title, Trunk, TaskBranch, Base, ArchiveCommit string
}

func InspectGitContext(root string) (GitContext, error) {
	data, populated, err := readPopulated(filepath.Join(root, ".concoct", "current", "task-plan.md"))
	if err != nil || !populated {
		return GitContext{}, err
	}
	var task taskMeta
	if err := parseFront(data, &task); err != nil {
		return GitContext{}, err
	}
	return GitContext{task.Git.Enabled, task.Git.Status == "archived", task.ID, task.Title, task.Git.Trunk, task.Git.TaskBranch, task.Git.Base, task.Git.ArchiveCommit}, nil
}

// InspectPromptContext reuses Detect as the authority for state validation and
// then returns the small additional context required for deterministic prompts.
func InspectPromptContext(root string) (PromptContext, error) {
	r := Detect(root)
	if r.OperationalError != nil {
		return PromptContext{}, r.OperationalError
	}
	if r.State == Invalid {
		return PromptContext{}, fmt.Errorf("invalid workflow state: %s", strings.Join(r.Diagnostics, "; "))
	}
	c := PromptContext{Report: r}
	if r.State != Ready && r.State != Integrating {
		gc, err := InspectGitContext(root)
		if err != nil {
			return PromptContext{}, err
		}
		if gc.Enabled {
			repo, ok, err := gitrepo.Open(root)
			if err != nil {
				return PromptContext{}, err
			}
			if !ok {
				return PromptContext{}, fmt.Errorf("task records Git metadata but project is not a Git repository")
			}
			if repo.OperationInProgress() {
				return PromptContext{}, fmt.Errorf("an unrelated Git operation is in progress")
			}
			branch, err := repo.Branch()
			if err != nil {
				return PromptContext{}, fmt.Errorf("detached HEAD is unsafe: %w", err)
			}
			if branch != gc.TaskBranch {
				return PromptContext{}, fmt.Errorf("checkout drift: expected task branch %s, found %s", gc.TaskBranch, branch)
			}
			if _, err := repo.Ref(gc.Trunk); err != nil {
				return PromptContext{}, fmt.Errorf("recorded Git trunk is unavailable: %w", err)
			}
			if _, err := repo.Ref(gc.Base); err != nil {
				return PromptContext{}, fmt.Errorf("recorded Git base is unavailable: %w", err)
			}
			head, err := repo.Head()
			if err != nil {
				return PromptContext{}, err
			}
			ancestor, err := repo.IsAncestor(gc.Base, head)
			if err != nil || !ancestor {
				return PromptContext{}, fmt.Errorf("recorded Git base is not an ancestor of the task branch")
			}
			if err := repo.Clean(); err != nil {
				return PromptContext{}, err
			}
		}
	}
	cur := filepath.Join(root, ".concoct", "current")
	reviews, _, err := readReviews(cur)
	if err != nil {
		return PromptContext{}, err
	}
	for _, review := range reviews {
		c.ReviewFiles = append(c.ReviewFiles, ".concoct/current/"+review.name)
	}
	c.NextReview = fmt.Sprintf(".concoct/current/review-%02d.md", len(reviews)+1)
	if r.State != Ready {
		data, _, err := readPopulated(filepath.Join(cur, "task-plan.md"))
		if err != nil {
			return PromptContext{}, err
		}
		var task taskMeta
		if err := parseFront(data, &task); err != nil {
			return PromptContext{}, err
		}
		c.TaskTitle = task.Title
		c.RemediatesReview = task.Remediates
		c.ResolutionRoute = task.Resolution.Route
	}
	return c, nil
}

// ValidatePlanItem verifies command-specific eligibility after Detect has
// established a ready repository with no conflicting active task.
func ValidatePlanItem(root, id string) error {
	_, err := InspectPlanEligibility(root, id)
	return err
}

// InspectPlanEligibility validates roadmap ordering and accepted capability
// truth while retaining limitations and provenance for planner judgment.
func InspectPlanEligibility(root, id string) (PlanEligibility, error) {
	if !regexp.MustCompile(`^[A-Z][A-Z0-9-]*-[0-9]+$`).MatchString(id) {
		return PlanEligibility{}, fmt.Errorf("invalid roadmap id %q; expected a stable identifier such as CON-006", id)
	}
	data, err := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
	if err != nil {
		return PlanEligibility{}, err
	}
	items, diagnostics := parseRoadmap(string(data))
	if len(diagnostics) > 0 {
		return PlanEligibility{}, fmt.Errorf("invalid roadmap: %s", strings.Join(diagnostics, "; "))
	}
	item, ok := items[id]
	if !ok {
		return PlanEligibility{}, fmt.Errorf("roadmap item %s does not exist", id)
	}
	if item.Status != "planned" {
		return PlanEligibility{}, fmt.Errorf("roadmap item %s is not eligible for planning (status %s); run concoct roadmap", id, item.Status)
	}
	for _, dependency := range item.Dependencies {
		dep, ok := items[dependency]
		if !ok || dep.Status != "delivered" {
			return PlanEligibility{}, fmt.Errorf("roadmap item %s has unsatisfied dependency %s; deliver it or record an explicit roadmap resolution", id, dependency)
		}
	}
	capData, err := os.ReadFile(filepath.Join(root, ".concoct", "capabilities.md"))
	if err != nil {
		return PlanEligibility{}, err
	}
	records, capDiagnostics := parseCapabilities(string(capData))
	if len(capDiagnostics) > 0 {
		return PlanEligibility{}, fmt.Errorf("roadmap item %s cannot validate capability prerequisites: %s; correct .concoct/capabilities.md before retrying planning", id, strings.Join(capDiagnostics, "; "))
	}
	result := PlanEligibility{}
	seen := map[string]bool{}
	for _, prerequisite := range item.Prerequisites {
		if seen[prerequisite] {
			return PlanEligibility{}, fmt.Errorf("roadmap item %s declares duplicate capability prerequisite %s; remove the duplicate in .concoct/roadmap.md", id, prerequisite)
		}
		seen[prerequisite] = true
		record, ok := records[prerequisite]
		if !ok {
			return PlanEligibility{}, fmt.Errorf("roadmap item %s capability prerequisite %s is missing from accepted capability truth; reconcile .concoct/capabilities.md or run concoct roadmap", id, prerequisite)
		}
		if record.Status != "active" {
			return PlanEligibility{}, fmt.Errorf("roadmap item %s capability prerequisite %s is not accepted active capability truth (status %s); reconcile the prerequisite before planning", id, prerequisite, record.Status)
		}
		result.Prerequisites = append(result.Prerequisites, PlanPrerequisite{prerequisite, record.Status, record.Limitations, record.Archives})
	}
	return result, nil
}

// InspectNextActionEvidence exposes all authoritative ready-state evidence in
// stable identifier order while delegating planning eligibility to the same
// validation used by concoct plan.
func InspectNextActionEvidence(root string) (NextActionEvidence, error) {
	r := Detect(root)
	if r.OperationalError != nil {
		return NextActionEvidence{}, r.OperationalError
	}
	if r.State != Ready {
		if r.State == Invalid {
			return NextActionEvidence{}, fmt.Errorf("invalid workflow state: %s", strings.Join(r.Diagnostics, "; "))
		}
		return NextActionEvidence{}, fmt.Errorf("concoct next is not valid in state %s; expected ready; run concoct status", r.State)
	}
	roadData, err := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
	if err != nil {
		return NextActionEvidence{}, err
	}
	items, diagnostics := parseRoadmap(string(roadData))
	if len(diagnostics) > 0 {
		return NextActionEvidence{}, fmt.Errorf("invalid roadmap: %s", strings.Join(diagnostics, "; "))
	}
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	evidence := NextActionEvidence{SupportedOrigins: []string{"human product input", "roadmap maintenance and reconciliation"}}
	for _, id := range ids {
		item := items[id]
		next := NextRoadmapItem{ID: id, Title: item.Title, Status: item.Status, Priority: item.Priority, Archive: item.Archive, Dependencies: item.Dependencies, Prerequisites: item.Prerequisites}
		if _, eligibilityErr := InspectPlanEligibility(root, id); eligibilityErr == nil {
			next.Eligible = true
		} else {
			next.Blocker = eligibilityErr.Error()
		}
		evidence.RoadmapItems = append(evidence.RoadmapItems, next)
	}
	capData, err := os.ReadFile(filepath.Join(root, ".concoct", "capabilities.md"))
	if err != nil {
		return NextActionEvidence{}, err
	}
	records, capDiagnostics := parseCapabilities(string(capData))
	if len(capDiagnostics) > 0 {
		return NextActionEvidence{}, fmt.Errorf("invalid capabilities: %s", strings.Join(capDiagnostics, "; "))
	}
	capIDs := make([]string, 0, len(records))
	for id := range records {
		capIDs = append(capIDs, id)
	}
	sort.Strings(capIDs)
	for _, id := range capIDs {
		record := records[id]
		evidence.Capabilities = append(evidence.Capabilities, NextCapability{ID: id, Status: record.Status, Limitations: record.Limitations, Archives: record.Archives})
	}
	return evidence, nil
}

func PlanItemTitle(root, id string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(id) + `\s+—\s+(.+?)\s*$`)
	m := re.FindStringSubmatch(string(b))
	if len(m) != 2 {
		return "", fmt.Errorf("roadmap item %s has no parseable title", id)
	}
	return strings.TrimSpace(m[1]), nil
}

var itemHeading = regexp.MustCompile(`(?m)^## ([A-Z][A-Z0-9-]*-[0-9]+)\s+—`)
var reviewName = regexp.MustCompile(`^review-([0-9]{2})\.md$`)

func Detect(root string) Report {
	r := Report{State: Invalid, Next: "repair the reported artifacts, then run concoct status"}
	roadData, err := os.ReadFile(filepath.Join(root, ".concoct", "roadmap.md"))
	if err != nil {
		r.OperationalError = fmt.Errorf("read .concoct/roadmap.md: %w", err)
		return r
	}
	capData, err := os.ReadFile(filepath.Join(root, ".concoct", "capabilities.md"))
	if err != nil {
		r.OperationalError = fmt.Errorf("read .concoct/capabilities.md: %w", err)
		return r
	}
	var roadHead, capHead map[string]any
	if err = parseFront(roadData, &roadHead); err != nil {
		r.Diagnostics = append(r.Diagnostics, ".concoct/roadmap.md: "+err.Error())
		return r
	}
	if err = parseFront(capData, &capHead); err != nil {
		r.Diagnostics = append(r.Diagnostics, ".concoct/capabilities.md: "+err.Error())
		return r
	}
	r.Project = stringValue(roadHead["project"])
	if r.Project == "" {
		r.Diagnostics = append(r.Diagnostics, ".concoct/roadmap.md: missing project metadata")
		return r
	}
	if cp := stringValue(capHead["project"]); cp == "" || cp != r.Project {
		r.Diagnostics = append(r.Diagnostics, ".concoct/capabilities.md: project metadata must match roadmap project")
		return r
	}
	items, diags := parseRoadmap(string(roadData))
	if len(diags) > 0 {
		r.Diagnostics = append(r.Diagnostics, diags...)
		return r
	}
	cur := filepath.Join(root, ".concoct", "current")
	taskPath := filepath.Join(cur, "task-plan.md")
	notesPath := filepath.Join(cur, "notes.md")
	taskData, taskPop, err := readPopulated(taskPath)
	if err != nil {
		r.OperationalError = err
		return r
	}
	_, notesPop, err := readNotes(notesPath)
	if err != nil {
		r.OperationalError = err
		return r
	}
	reviews, revDiags, opErr := readReviews(cur)
	if opErr != nil {
		r.OperationalError = opErr
		return r
	}
	if len(revDiags) > 0 {
		r.Diagnostics = append(r.Diagnostics, revDiags...)
		return r
	}
	if !taskPop && !notesPop {
		if len(reviews) > 0 {
			r.Diagnostics = append(r.Diagnostics, ".concoct/current: reviews exist without an active task")
			return r
		}
		recovery, err := filepath.Glob(filepath.Join(root, ".git", "concoct", "integrations", "*.yaml"))
		if err != nil {
			r.OperationalError = err
			return r
		}
		if len(recovery) > 0 {
			r.State = Integrated
			r.Diagnostics = append(r.Diagnostics, "integration recovery remains after active-state cleanup")
			r.Next = "concoct integrate --continue"
			return r
		}
		r.State = Ready
		r.Next = "concoct next"
		return r
	}
	if taskPop != notesPop {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md and notes.md must both be populated")
		return r
	}
	var task taskMeta
	if err = parseFront(taskData, &task); err != nil {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: "+err.Error())
		return r
	}
	r.RoadmapItem = task.RoadmapID
	r.TaskStatus = task.Status
	r.CapabilityImpact = task.Impact.Type
	if task.Git.Enabled {
		r.GitTrunk, r.GitTaskBranch, r.GitArchiveCommit = task.Git.Trunk, task.Git.TaskBranch, task.Git.ArchiveCommit
		if task.Git.Trunk == "" || task.Git.TaskBranch == "" || task.Git.Base == "" {
			r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: enabled git metadata requires trunk, task-branch, and base")
			return r
		}
		switch task.Git.Status {
		case "", "active", "archived", "integrating", "integrated":
		default:
			r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: unknown git.status "+task.Git.Status)
			return r
		}
	}
	if task.ID == "" || task.RoadmapID == "" || task.ID != task.RoadmapID {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: fields id and roadmap-id must be present and equal")
		return r
	}
	if task.Title == "" {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: missing required field title")
		return r
	}
	if task.Created == "" {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: missing required field created")
		return r
	}
	if task.Updated == "" {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: missing required field updated")
		return r
	}
	item, ok := items[task.RoadmapID]
	if !ok {
		r.Diagnostics = append(r.Diagnostics, "roadmap item "+task.RoadmapID+" does not exist")
		return r
	}
	if item.Status != "active" {
		r.Diagnostics = append(r.Diagnostics, "roadmap item "+task.RoadmapID+" must have Status active while its task is current (found "+item.Status+")")
		return r
	}
	if !validImpact(task.Impact) {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: capability-impact must use add, update, remove, or none and include IDs when applicable")
		return r
	}
	if strings.TrimSpace(task.Impact.Rationale) == "" {
		r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: missing required field capability-impact.rationale")
		return r
	}
	if task.Status != "planned" && task.Status != "implementation-in-progress" && task.Status != "implementation-complete" {
		r.Diagnostics = append(r.Diagnostics, "unknown task status "+task.Status)
		return r
	}
	if len(reviews) == 0 {
		r.State = mapTask(task.Status)
		r.Next = next(r.State)
		return r
	}
	latest := reviews[len(reviews)-1]
	r.LatestReview = latest.name
	r.ReviewOutcome = latest.meta.Status
	if latest.meta.TaskID != task.ID {
		r.Diagnostics = append(r.Diagnostics, latest.name+": task-id does not match active task")
		return r
	}
	if task.Git.Enabled && task.Git.Status != "" && task.Git.Status != "active" {
		if latest.meta.Status != "approved" || task.Status != "implementation-complete" {
			r.Diagnostics = append(r.Diagnostics, "Git archival/integration evidence requires an approved implementation-complete task")
			return r
		}
		if task.Git.ArchiveCommit == "" {
			r.Diagnostics = append(r.Diagnostics, ".concoct/current/task-plan.md: archived Git task requires archive-commit")
			return r
		}
		if _, err := os.Stat(filepath.Join(root, ".git", "concoct", "integrations", task.ID+".yaml")); err == nil || task.Git.Status == "integrating" {
			r.State, r.Next = Integrating, "resolve and stage conflicts, then run concoct integrate --continue, or run concoct integrate --abort"
			return r
		}
		if task.Git.Status == "integrated" {
			r.State, r.Next = Integrated, "concoct integrate --continue"
			return r
		}
		r.State, r.Next = Archived, "concoct integrate"
		return r
	}
	if task.Status == "planned" {
		r.Diagnostics = append(r.Diagnostics, "task status planned cannot coexist with reviews")
		return r
	}
	if task.Remediates != "" {
		if task.Remediates != latest.name {
			if !historicalReview(reviews, task.Remediates, "changes-requested") {
				r.Diagnostics = append(r.Diagnostics, "remediates-review must name the latest changes-requested review")
				return r
			}
			// A later review supersedes retained remediation history. Continue so
			// recovery evidence for that later review can still be evaluated.
		} else if latest.meta.Status != "changes-requested" {
			r.Diagnostics = append(r.Diagnostics, "remediates-review must name the latest changes-requested review")
			return r
		} else if task.Status == "implementation-complete" && !hasDispositions(string(mustRead(notesPath)), latest.body) {
			r.Diagnostics = append(r.Diagnostics, "notes.md lacks completed dispositions for findings in "+latest.name)
			return r
		} else {
			r.State = mapTask(task.Status)
			r.Next = next(r.State)
			return r
		}
	}
	if task.Resolution.Review != "" {
		if task.Resolution.Review != latest.name {
			if !historicalReview(reviews, task.Resolution.Review, "blocked") {
				r.Diagnostics = append(r.Diagnostics, "blocked-review-resolution must name the latest blocked review")
				return r
			}
			// A later completed review supersedes retained blocker-resolution history.
		} else if d := validateResolution(root, task, latest, string(mustRead(notesPath))); d != "" {
			r.Diagnostics = append(r.Diagnostics, d)
			return r
		} else {
			r.State = mapTask(task.Status)
			r.Next = next(r.State)
			return r
		}
	}

	if task.Status != "implementation-complete" {
		r.Diagnostics = append(r.Diagnostics, "task status must remain implementation-complete while latest review is authoritative")
		return r
	}
	switch latest.meta.Status {
	case "changes-requested":
		r.State = ChangesRequested
	case "approved":
		r.State = Approved
	case "blocked":
		r.State = Blocked
	default:
		r.Diagnostics = append(r.Diagnostics, latest.name+": unknown review outcome "+latest.meta.Status)
		return r
	}
	r.Next = next(r.State)
	return r
}

func historicalReview(reviews []review, name, outcome string) bool {
	for _, r := range reviews[:len(reviews)-1] {
		if r.name == name && r.meta.Status == outcome {
			return true
		}
	}
	return false
}

type review struct {
	name, body string
	meta       reviewMeta
}

func readReviews(cur string) ([]review, []string, error) {
	ents, err := os.ReadDir(cur)
	if err != nil {
		return nil, nil, fmt.Errorf("read .concoct/current: %w", err)
	}
	var names []string
	var diags []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "review-") {
			if !reviewName.MatchString(e.Name()) {
				diags = append(diags, e.Name()+": review filename must be zero-padded review-NN.md")
			} else {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	out := make([]review, 0, len(names))
	for i, n := range names {
		want := i + 1
		m := reviewName.FindStringSubmatch(n)
		num, _ := strconv.Atoi(m[1])
		if num != want {
			diags = append(diags, fmt.Sprintf("%s: review sequence gap; expected review-%02d.md", n, want))
		}
		data, err := os.ReadFile(filepath.Join(cur, n))
		if err != nil {
			return nil, nil, err
		}
		var meta reviewMeta
		if err = parseFront(data, &meta); err != nil {
			diags = append(diags, n+": "+err.Error())
			continue
		}
		if meta.Review != num {
			diags = append(diags, n+": internal review number does not match filename")
		}
		if meta.TaskID == "" {
			diags = append(diags, n+": missing task-id")
		}
		if meta.Created == "" {
			diags = append(diags, n+": missing required field created")
		}
		if meta.Persona == "" {
			diags = append(diags, n+": missing required field persona")
		} else if meta.Persona != "reviewer" {
			diags = append(diags, n+": field persona must be reviewer")
		}
		if meta.Status == reservationStatus {
			if !strings.Contains(string(data), "Replace status: reserved") || strings.Contains(string(data), "## Outcome") {
				diags = append(diags, n+": malformed review reservation; restore the generated reservation or complete the review")
			}
			continue
		}
		if meta.Status != "approved" && meta.Status != "changes-requested" && meta.Status != "blocked" {
			diags = append(diags, n+": outcome must be approved, changes-requested, or blocked")
		}
		if outcome, count := documentedOutcome(string(data)); count != 1 || outcome != meta.Status {
			diags = append(diags, n+": must document exactly one Outcome matching front matter status")
		}
		out = append(out, review{name: n, body: string(data), meta: meta})
	}
	return out, diags, nil
}

func documentedOutcome(body string) (string, int) {
	start := strings.Index(body, "## Outcome")
	if start < 0 {
		return "", 0
	}
	section := body[start+len("## Outcome"):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	found := ""
	count := 0
	for _, outcome := range []string{"approved", "changes-requested", "blocked"} {
		n := strings.Count(section, "`"+outcome+"`")
		if n > 0 {
			found = outcome
			count += n
		}
	}
	return found, count
}
func parseFront(data []byte, out any) error {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return fmt.Errorf("missing YAML front matter")
	}
	end := bytes.Index(data[4:], []byte("\n---\n"))
	if end < 0 {
		return fmt.Errorf("unterminated YAML front matter")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data[4 : 4+end]))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("malformed YAML front matter: %w", err)
	}
	return nil
}
func parseRoadmap(s string) (map[string]roadItem, []string) {
	matches := itemHeading.FindAllStringSubmatchIndex(s, -1)
	items := map[string]roadItem{}
	var d []string
	for i, m := range matches {
		id := s[m[2]:m[3]]
		end := len(s)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := s[m[0]:end]
		re := regexp.MustCompile("(?m)^- Status: `?([^`\\n]+)`?\\s*$")
		sm := re.FindStringSubmatch(section)
		if len(sm) != 2 {
			d = append(d, ".concoct/roadmap.md: "+id+" missing Status")
			continue
		}
		if _, exists := items[id]; exists {
			d = append(d, ".concoct/roadmap.md: duplicate item "+id)
		}
		title := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(section, "\n", 2)[0], "## "+id+" —"))
		item := roadItem{Title: title, Status: strings.TrimSpace(sm[1])}
		if pm := regexp.MustCompile("(?m)^- Priority: `?([^`\\n]+)`?\\s*$").FindStringSubmatch(section); len(pm) == 2 {
			item.Priority = strings.TrimSpace(pm[1])
		}
		if am := regexp.MustCompile("(?m)^- Archive:\\s*(`?\\.concoct/archive/[^`\\s]+`?)").FindStringSubmatch(section); len(am) == 2 {
			item.Archive = strings.TrimSuffix(strings.Trim(am[1], "`"), "/") + "/summary.md"
		}
		depRE := regexp.MustCompile(`(?m)^- Depends on:\s*(.+?)\s*$`)
		if dm := depRE.FindStringSubmatch(section); len(dm) == 2 {
			value := strings.Trim(strings.TrimSpace(dm[1]), "`")
			if value != "" && strings.ToLower(value) != "none" {
				for _, dependency := range strings.Split(value, ",") {
					item.Dependencies = append(item.Dependencies, strings.TrimSpace(dependency))
				}
			}
		}
		prereqRE := regexp.MustCompile(`(?m)^- Capability prerequisites:\s*(.+?)\s*$`)
		if pm := prereqRE.FindStringSubmatch(section); len(pm) == 2 {
			value := strings.Trim(strings.TrimSpace(pm[1]), "`")
			if value != "" && strings.ToLower(value) != "none" {
				for _, prerequisite := range strings.Split(value, ",") {
					prerequisite = strings.TrimSpace(strings.Trim(prerequisite, "`"))
					if !regexp.MustCompile(`^CAP-[0-9]+$`).MatchString(prerequisite) {
						d = append(d, ".concoct/roadmap.md: "+id+" has malformed Capability prerequisite "+prerequisite)
					}
					item.Prerequisites = append(item.Prerequisites, prerequisite)
				}
			}
		}
		items[id] = item
	}
	active := 0
	validStatuses := map[string]bool{"candidate": true, "planned": true, "active": true, "blocked": true, "delivered": true, "deferred": true, "cancelled": true}
	for id, v := range items {
		if !validStatuses[v.Status] {
			d = append(d, ".concoct/roadmap.md: "+id+" has unknown Status "+v.Status)
		}
		if v.Status == "active" {
			active++
		}
	}
	if active > 1 {
		d = append(d, ".concoct/roadmap.md: multiple active roadmap items")
	}
	return items, d
}

var capabilityHeading = regexp.MustCompile(`(?m)^## (CAP-[0-9]+)\s+—`)

func parseCapabilities(s string) (map[string]capabilityRecord, []string) {
	matches := capabilityHeading.FindAllStringSubmatchIndex(s, -1)
	records := map[string]capabilityRecord{}
	var diagnostics []string
	for i, match := range matches {
		id := s[match[2]:match[3]]
		end := len(s)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		section := s[match[0]:end]
		if _, exists := records[id]; exists {
			diagnostics = append(diagnostics, ".concoct/capabilities.md: duplicate capability "+id)
			continue
		}
		statusMatch := regexp.MustCompile("(?m)^- Status: `?([^`\\n]+)`?\\s*$").FindStringSubmatch(section)
		if len(statusMatch) != 2 {
			diagnostics = append(diagnostics, ".concoct/capabilities.md: "+id+" missing Status")
			continue
		}
		record := capabilityRecord{Status: strings.TrimSpace(statusMatch[1])}
		if limitations := regexp.MustCompile(`(?ms)^### Limitations\s*\n(.*?)(?:\n### |\z)`).FindStringSubmatch(section); len(limitations) == 2 {
			record.Limitations = strings.TrimSpace(limitations[1])
		}
		archiveRE := regexp.MustCompile(`\.concoct/archive/[^/` + "`" + `\s]+`)
		for _, archive := range archiveRE.FindAllString(section, -1) {
			record.Archives = append(record.Archives, archive+"/summary.md")
		}
		sort.Strings(record.Archives)
		if len(record.Archives) > 1 {
			compacted := record.Archives[:1]
			for _, archive := range record.Archives[1:] {
				if archive != compacted[len(compacted)-1] {
					compacted = append(compacted, archive)
				}
			}
			record.Archives = compacted
		}
		records[id] = record
	}
	return records, diagnostics
}
func readPopulated(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	trim := strings.TrimSpace(string(data))
	return data, strings.HasPrefix(trim, "---"), nil
}

func readNotes(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	text := strings.TrimSpace(string(data))
	placeholder := strings.HasPrefix(text, "# notes.md") && strings.Contains(text, "_Add decisions here._") && strings.Contains(text, "_Record meaningful verification results here._")
	return data, text != "" && !placeholder, nil
}
func validImpact(i impact) bool {
	switch i.Type {
	case "none":
		return len(i.IDs) == 0
	case "add", "update", "remove":
		return len(i.IDs) > 0
	default:
		return false
	}
}
func mapTask(s string) State {
	if s == "planned" {
		return Planned
	}
	if s == "implementation-in-progress" {
		return InProgress
	}
	return Complete
}
func next(s State) string {
	switch s {
	case Planned, InProgress, ChangesRequested:
		return "concoct code"
	case Complete:
		return "concoct review"
	case Approved:
		return "concoct archive"
	case Blocked:
		return "route the blocker to the responsible role or human"
	default:
		return ""
	}
}
func validateResolution(root string, t taskMeta, r review, notes string) string {
	x := t.Resolution
	if x.Review != r.name || r.meta.Status != "blocked" {
		return "blocked-review-resolution must name the latest blocked review"
	}
	if x.Route != "code" && x.Route != "review" {
		return "blocked-review-resolution route must be code or review"
	}
	if x.RecordedBy != "task-planner" && x.RecordedBy != "developer" {
		return "blocked-review-resolution recorded-by is unauthorized"
	}
	if (x.Route == "code" && t.Status != "implementation-in-progress") || (x.Route == "review" && t.Status != "implementation-complete") {
		return "blocked-review-resolution route disagrees with task status"
	}
	if len(x.Evidence) == 0 {
		return "blocked-review-resolution evidence must not be empty"
	}
	for _, p := range x.Evidence {
		if filepath.IsAbs(p) || strings.Contains(p, "..") || strings.ContainsAny(p, "*?#") || strings.Contains(p, "://") {
			return "blocked-review-resolution contains an unsafe evidence path"
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
		if err != nil || !info.Mode().IsRegular() {
			return "blocked-review-resolution evidence does not name a readable file: " + p
		}
	}
	if !strings.Contains(strings.ToLower(notes), "block") || !strings.Contains(strings.ToLower(notes), "handoff to reviewer") && x.Route == "review" {
		return "notes.md lacks the required blocker disposition or fresh reviewer handoff"
	}
	return ""
}
func hasDispositions(notes, review string) bool {
	count := strings.Count(review, "### Finding ")
	if count == 0 {
		return true
	}
	lower := strings.ToLower(notes)
	words := []string{"fixed", "partially fixed", "disputed", "obsolete", "blocked"}
	n := 0
	for _, w := range words {
		n += strings.Count(lower, w)
	}
	return n >= count
}
func mustRead(path string) []byte { b, _ := os.ReadFile(path); return b }
func stringValue(v any) string    { s, _ := v.(string); return s }
