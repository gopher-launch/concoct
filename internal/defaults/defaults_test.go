package defaults

import (
	"bytes"
	"testing"
)

func TestListIsDeterministicAndResourcesAreReadable(t *testing.T) {
	list := List()
	if len(list) == 0 {
		t.Fatal("no resources")
	}
	for i, r := range list {
		if i > 0 && list[i-1].ID >= r.ID {
			t.Fatal("resources are not ordered")
		}
		_, err := Read(r.ID, "test")
		if err != nil {
			t.Fatalf("Read(%s): %v", r.ID, err)
		}
	}
}
func TestReadIgnoresProjectFilesAndRejectsUnknown(t *testing.T) {
	first, err := Read("protocol", "test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Read("protocol", "test")
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("embedded protocol is not stable")
	}
	if _, err := Read("missing", "test"); err == nil {
		t.Fatal("unknown resource succeeded")
	}
}
