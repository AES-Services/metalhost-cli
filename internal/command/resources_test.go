package command

import "testing"

func TestLookupResourceByKindAndAlias(t *testing.T) {
	cases := map[string]string{
		"vm":              "vm",
		"vms":             "vm",
		"virtual-machine": "vm",
		"VM":              "vm",
		"disks":           "disk",
		"share":           "file-share",
		"subscription":    "webhook",
		"proj":            "project",
	}
	for input, wantKind := range cases {
		h, ok := lookupResource(input)
		if !ok {
			t.Errorf("lookupResource(%q) not found", input)
			continue
		}
		if h.kind != wantKind {
			t.Errorf("lookupResource(%q).kind = %q, want %q", input, h.kind, wantKind)
		}
	}
}

func TestLookupResourceUnknown(t *testing.T) {
	if _, ok := lookupResource("nope"); ok {
		t.Fatal("lookupResource(\"nope\") should not be found")
	}
}

func TestEveryResourceHasListAndParent(t *testing.T) {
	for _, h := range resourceRegistry() {
		if h.list == nil {
			t.Errorf("resource %q has no list handler", h.kind)
		}
		if h.title == "" {
			t.Errorf("resource %q has no title", h.kind)
		}
	}
}

func TestResourceKindsNonEmpty(t *testing.T) {
	if len(resourceKinds()) == 0 {
		t.Fatal("resourceKinds() is empty")
	}
}
