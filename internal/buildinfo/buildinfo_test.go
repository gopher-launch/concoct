package buildinfo

import "testing"

func TestOfficialRequiresCompleteCleanReleaseMetadata(t *testing.T) {
	original := []string{Version, Revision, Dirty, Release}
	t.Cleanup(func() { Version, Revision, Dirty, Release = original[0], original[1], original[2], original[3] })
	Version, Revision, Dirty, Release = "v0.4.2", "abcdef012345", "false", "true"
	if !Current().Official {
		t.Fatal("complete clean tag was not official")
	}
	for _, bad := range [][]string{{"v0.4", "abcdef0", "false", "true"}, {"v0.4.2", "unknown", "false", "true"}, {"v0.4.2", "abcdef0", "true", "true"}, {"v0.4.2", "abcdef0", "false", "false"}} {
		Version, Revision, Dirty, Release = bad[0], bad[1], bad[2], bad[3]
		if Current().Official {
			t.Fatalf("metadata %v was official", bad)
		}
	}
}
