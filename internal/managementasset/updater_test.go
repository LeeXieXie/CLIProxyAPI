package managementasset

import "testing"

func TestResolveReleaseURL_DefaultsToCPAManager(t *testing.T) {
	got := resolveReleaseURL("")
	want := "https://api.github.com/repos/seakee/CPA-Manager/releases/latest"
	if got != want {
		t.Fatalf("resolveReleaseURL(empty) = %q, want %q", got, want)
	}
}

func TestResolveReleaseURL_MapsGitHubRepositoryToLatestRelease(t *testing.T) {
	got := resolveReleaseURL("https://github.com/seakee/CPA-Manager")
	want := "https://api.github.com/repos/seakee/CPA-Manager/releases/latest"
	if got != want {
		t.Fatalf("resolveReleaseURL(CPA-Manager) = %q, want %q", got, want)
	}
}
