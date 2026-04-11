package managementasset

import (
	"strings"
	"testing"
)

func TestExtensionAssetURL_UsesVersionedRoute(t *testing.T) {
	got := ExtensionAssetURL("bootstrap.js")
	if !strings.HasPrefix(got, ExtensionAssetRoutePrefix+"/bootstrap.js?v=") {
		t.Fatalf("asset url = %q", got)
	}
	if strings.HasSuffix(got, "?v=") {
		t.Fatalf("asset url missing version: %q", got)
	}
}

func TestLookupExtensionAsset_ReturnsEmbeddedAssets(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		contains    string
	}{
		{name: "bootstrap.js", contentType: "application/javascript", contains: "window.__cpaBootstrapLoaded"},
		{name: "usage.plugin.js", contentType: "application/javascript", contains: "window.__cpaUsagePluginLoaded"},
		{name: "ai-providers.plugin.js", contentType: "application/javascript", contains: "window.__cpaAIProvidersPluginLoaded"},
		{name: "extensions.css", contentType: "text/css", contains: "#cpa-fab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asset, ok := LookupExtensionAsset(tt.name)
			if !ok {
				t.Fatalf("expected asset %q", tt.name)
			}
			if got := asset.Name; got != tt.name {
				t.Fatalf("asset name = %q, want %q", got, tt.name)
			}
			if got := asset.ContentType; !strings.Contains(got, tt.contentType) {
				t.Fatalf("content-type = %q, want contains %q", got, tt.contentType)
			}
			if body := string(asset.Content); !strings.Contains(body, tt.contains) {
				t.Fatalf("asset %q missing %q", tt.name, tt.contains)
			}
		})
	}
}

func TestLookupExtensionAsset_RejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"", "missing.js", "../bootstrap.js", "nested/bootstrap.js", "/../extensions.css"} {
		if asset, ok := LookupExtensionAsset(name); ok {
			t.Fatalf("unexpected asset for %q: %+v", name, asset)
		}
		if got := ExtensionAssetURL(name); got != "" {
			t.Fatalf("asset url for %q = %q, want empty", name, got)
		}
	}
}
