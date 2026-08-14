package apidocs

import (
	"strings"
	"testing"
)

func TestVersionedDocumentation(t *testing.T) {
	versions := List()
	if len(versions) != 1 || versions[0].Version != "v1" || versions[0].Status != "current" {
		t.Fatalf("unexpected versions: %+v", versions)
	}

	meta, body, err := Get("v1")
	if err != nil {
		t.Fatal(err)
	}
	if meta.ContentType != "text/markdown" || !strings.Contains(body, "DELETE /admin/api/projects/{id}") || !strings.Contains(body, "project_has_dependencies") {
		t.Fatal("v1 documentation is missing the deletion contract")
	}

	if _, _, err := Get("v2"); err == nil {
		t.Fatal("unsupported documentation version was accepted")
	}
}
