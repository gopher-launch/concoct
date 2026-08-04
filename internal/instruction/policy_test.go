package instruction

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyHasExplicitDispositionForEveryActivity(t *testing.T) {
	root := fixture(t, "")
	effective, err := Compose(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, activity := range []Activity{ProductOwnership, TaskPlanning, Development, IndependentReview, Archival, Integration} {
		if got := effective.Policy.Requirements[activity]; got != Required && got != NotRequired {
			t.Fatalf("%s = %q", activity, got)
		}
	}
}

func TestPolicyRejectsUnknownAndInconsistentSelections(t *testing.T) {
	for name, body := range map[string]string{
		"unknown activity":           "required-phases:\n  - development\n  - archival\n  - invented\napproval-gates: []\ngit-strategy: task-branch-with-squash-integration\n",
		"review gate without review": "required-phases:\n  - development\n  - archival\napproval-gates:\n  - reviewer-approval-before-archive\ngit-strategy: task-branch-with-squash-integration\n",
		"review without gate":        "required-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - independent-review\n  - archival\n  - integration\napproval-gates:\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n",
		"unsupported planning skip":  "required-phases:\n  - product-ownership\n  - development\n  - archival\n  - integration\nnot-required-reasons:\n  - task-planning: supplied externally\napproval-gates:\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := fixture(t, "")
			if err := os.WriteFile(filepath.Join(root, PolicyPath), []byte("---\ninstruction-layer: policy\n"+body+"---\n# Policy\n"), 0644); err != nil {
				t.Fatal(err)
			}
			_, err := Compose(root)
			if err == nil || !strings.Contains(err.Error(), "policy") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPolicyRequiresDurableReasonForEveryOmission(t *testing.T) {
	root := fixture(t, "")
	policy := "---\ninstruction-layer: policy\nrequired-phases:\n  - product-ownership\n  - task-planning\n  - development\n  - archival\n  - integration\nnot-required-reasons:\n  - independent-review: external audit is required\napproval-gates:\n  - archive-before-integration\ngit-strategy: task-branch-with-squash-integration\n---\n# Policy\n"
	if err := os.WriteFile(filepath.Join(root, PolicyPath), []byte(policy), 0644); err != nil {
		t.Fatal(err)
	}
	effective, err := Compose(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := effective.Policy.Reasons[IndependentReview]; got != "external audit is required" {
		t.Fatalf("reason = %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, PolicyPath), []byte(strings.Replace(policy, "  - independent-review: external audit is required\n", "", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Compose(root); err == nil || !strings.Contains(err.Error(), "needs a not-required-reasons") {
		t.Fatalf("error = %v", err)
	}
}
