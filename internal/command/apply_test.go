package command

import (
	"testing"

	storagev1 "github.com/AES-Services/metalhost-sdk/gen/go/aes/storage/v1"
)

func TestParseManifest(t *testing.T) {
	t.Run("yaml ok", func(t *testing.T) {
		m, err := parseManifest([]byte("kind: Disk\nparent: projects/p\nspec:\n  sizeGib: 50\n"))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if m.Kind != "Disk" || m.Parent != "projects/p" {
			t.Fatalf("unexpected envelope: %+v", m)
		}
		if m.Spec["sizeGib"] == nil {
			t.Fatalf("spec not decoded: %+v", m.Spec)
		}
	})
	t.Run("json ok", func(t *testing.T) {
		if _, err := parseManifest([]byte(`{"kind":"Disk","spec":{"sizeGib":50}}`)); err != nil {
			t.Fatalf("json parse: %v", err)
		}
	})
	t.Run("missing kind", func(t *testing.T) {
		if _, err := parseManifest([]byte("spec: {}\n")); err == nil {
			t.Fatal("expected error for missing kind")
		}
	})
	t.Run("missing spec", func(t *testing.T) {
		if _, err := parseManifest([]byte("kind: Disk\n")); err == nil {
			t.Fatal("expected error for missing spec")
		}
	})
}

func TestFindMessageByName(t *testing.T) {
	for _, name := range []string{"Disk", "disk", "DISK"} {
		md, err := findMessageByName(name)
		if err != nil {
			t.Fatalf("findMessageByName(%q): %v", name, err)
		}
		if md.Name() != "Disk" {
			t.Fatalf("findMessageByName(%q) = %s, want Disk", name, md.Name())
		}
	}
	if _, err := findMessageByName("Nope"); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}

func TestBuildCreateRequest(t *testing.T) {
	resDesc := (&storagev1.Disk{}).ProtoReflect().Descriptor()
	res := (&storagev1.Disk{DatacenterName: "datacenters/d", SizeGib: 50, StorageClass: "nvme"}).ProtoReflect()

	md, req, err := buildCreateRequest(resDesc, res.Interface(), "projects/p")
	if err != nil {
		t.Fatalf("buildCreateRequest: %v", err)
	}
	if md.Name() != "CreateDisk" {
		t.Fatalf("method = %s, want CreateDisk", md.Name())
	}
	rm := req.ProtoReflect()
	parentFD := rm.Descriptor().Fields().ByName("parent")
	if got := rm.Get(parentFD).String(); got != "projects/p" {
		t.Fatalf("parent = %q, want projects/p", got)
	}
	diskFD := rm.Descriptor().Fields().ByName("disk")
	embedded := rm.Get(diskFD).Message()
	scFD := embedded.Descriptor().Fields().ByName("storage_class")
	if got := embedded.Get(scFD).String(); got != "nvme" {
		t.Fatalf("disk.storage_class = %q, want nvme", got)
	}
}

func TestBuildCreateRequestMissingParent(t *testing.T) {
	resDesc := (&storagev1.Disk{}).ProtoReflect().Descriptor()
	res := (&storagev1.Disk{SizeGib: 1}).ProtoReflect()
	if _, _, err := buildCreateRequest(resDesc, res.Interface(), ""); err == nil {
		t.Fatal("expected error when parent is empty")
	}
}

func TestBuildUpdateRequestMask(t *testing.T) {
	resDesc := (&storagev1.Disk{}).ProtoReflect().Descriptor()
	res := (&storagev1.Disk{Name: "projects/p/disks/d", DisplayName: "renamed"}).ProtoReflect()

	md, req, err := buildUpdateRequest(resDesc, res.Interface())
	if err != nil {
		t.Fatalf("buildUpdateRequest: %v", err)
	}
	if md.Name() != "UpdateDisk" {
		t.Fatalf("method = %s, want UpdateDisk", md.Name())
	}
	rm := req.ProtoReflect()
	maskFD := rm.Descriptor().Fields().ByName("update_mask")
	mask := rm.Get(maskFD).Message()
	pathsFD := mask.Descriptor().Fields().ByName("paths")
	paths := mask.Get(pathsFD).List()

	got := map[string]bool{}
	for i := 0; i < paths.Len(); i++ {
		got[paths.Get(i).String()] = true
	}
	if !got["display_name"] {
		t.Fatalf("mask missing display_name: %v", got)
	}
	if got["name"] {
		t.Fatalf("mask should not include the identifier `name`: %v", got)
	}
}

func TestUnwrapSingleResource(t *testing.T) {
	// GetDiskResponse{disk} → bare Disk.
	resp := &storagev1.GetDiskResponse{Disk: &storagev1.Disk{Name: "projects/p/disks/d", DisplayName: "x"}}
	got := unwrapSingleResource(resp)
	if got.ProtoReflect().Descriptor().Name() != "Disk" {
		t.Fatalf("unwrap = %s, want Disk", got.ProtoReflect().Descriptor().Name())
	}

	// A multi-field message is returned unchanged.
	multi := &storagev1.ListDisksResponse{}
	if unwrapSingleResource(multi).ProtoReflect().Descriptor().Name() != "ListDisksResponse" {
		t.Fatal("multi-field message should not be unwrapped")
	}
}

func TestSetFieldNamesExcludesName(t *testing.T) {
	res := (&storagev1.Disk{Name: "n", DisplayName: "d", StorageClass: "nvme"}).ProtoReflect()
	names := setFieldNames(res.Interface(), "name")
	for _, n := range names {
		if n == "name" {
			t.Fatalf("setFieldNames should exclude name: %v", names)
		}
	}
	var hasDisplay, hasClass bool
	for _, n := range names {
		switch n {
		case "display_name":
			hasDisplay = true
		case "storage_class":
			hasClass = true
		}
	}
	if !hasDisplay || !hasClass {
		t.Fatalf("expected display_name and storage_class in %v", names)
	}
}