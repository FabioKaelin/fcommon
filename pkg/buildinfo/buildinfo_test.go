package buildinfo

import "testing"

func TestLinkedVersionCannotBeOverridden(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "1.3.0"
	t.Setenv("F_VERSION", "wrong")
	if Current("old").Version != "1.3.0" {
		t.Fatal("immutable version overridden")
	}
	Version = "dev"
	if Current("old").Version != "wrong" {
		t.Fatal("development override missing")
	}
}
