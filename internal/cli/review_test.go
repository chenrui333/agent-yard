package cli

import "testing"

func TestPRCheckoutPreviewIncludesRepoArg(t *testing.T) {
	got := prCheckoutPreview("owner/repo", 123)
	want := "gh pr checkout 123 --detach --repo owner/repo"
	if got != want {
		t.Fatalf("prCheckoutPreview = %q; want %q", got, want)
	}
}
