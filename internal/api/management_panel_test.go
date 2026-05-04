package api

import (
	"bytes"
	"testing"
)

func TestPatchManagementControlPanelHTMLRewritesModelFetch(t *testing.T) {
	input := []byte("prefix " + managementPanelModelFetchSnippet + " suffix")

	got := patchManagementControlPanelHTML(input)

	if bytes.Contains(got, []byte(managementPanelModelFetchSnippet)) {
		t.Fatalf("expected direct /v1 model fetch snippet to be replaced: %s", got)
	}
	if !bytes.Contains(got, []byte(managementPanelModelFetchPatch)) {
		t.Fatalf("expected management model fetch patch to be present: %s", got)
	}
}

func TestPatchManagementControlPanelHTMLNoopWhenSnippetMissing(t *testing.T) {
	input := []byte("unchanged")

	got := patchManagementControlPanelHTML(input)

	if !bytes.Equal(got, input) {
		t.Fatalf("expected HTML without known snippet to remain unchanged: %s", got)
	}
}
