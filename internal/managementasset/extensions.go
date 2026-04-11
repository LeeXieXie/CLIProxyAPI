package managementasset

import (
	_ "embed"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const ExtensionAssetRoutePrefix = "/-/management-assets"

type ExtensionAsset struct {
	Name        string
	Content     []byte
	ContentType string
}

var (
	//go:embed assets/bootstrap.js
	bootstrapJS []byte

	//go:embed assets/usage.plugin.js
	usagePluginJS []byte

	//go:embed assets/ai-providers.plugin.js
	aiProvidersPluginJS []byte

	//go:embed assets/extensions.css
	extensionsCSS []byte

	extensionAssetsVersion = buildExtensionAssetsVersion()
)

func buildExtensionAssetsVersion() string {
	h := sha256.New()
	_, _ = h.Write(bootstrapJS)
	_, _ = h.Write(usagePluginJS)
	_, _ = h.Write(aiProvidersPluginJS)
	_, _ = h.Write(extensionsCSS)
	sum := hex.EncodeToString(h.Sum(nil))
	if len(sum) > 12 {
		return sum[:12]
	}
	return sum
}

func ExtensionAssetsVersion() string {
	return extensionAssetsVersion
}

func ExtensionAssetURL(name string) string {
	cleaned := normalizeExtensionAssetName(name)
	if cleaned == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s?v=%s", ExtensionAssetRoutePrefix, cleaned, ExtensionAssetsVersion())
}

func LookupExtensionAsset(name string) (ExtensionAsset, bool) {
	switch normalizeExtensionAssetName(name) {
	case "bootstrap.js":
		return ExtensionAsset{Name: "bootstrap.js", Content: bootstrapJS, ContentType: "application/javascript; charset=utf-8"}, true
	case "usage.plugin.js":
		return ExtensionAsset{Name: "usage.plugin.js", Content: usagePluginJS, ContentType: "application/javascript; charset=utf-8"}, true
	case "ai-providers.plugin.js":
		return ExtensionAsset{Name: "ai-providers.plugin.js", Content: aiProvidersPluginJS, ContentType: "application/javascript; charset=utf-8"}, true
	case "extensions.css":
		return ExtensionAsset{Name: "extensions.css", Content: extensionsCSS, ContentType: "text/css; charset=utf-8"}, true
	default:
		return ExtensionAsset{}, false
	}
}

func normalizeExtensionAssetName(name string) string {
	cleaned := strings.TrimPrefix(strings.TrimSpace(name), "/")
	if cleaned == "" || strings.Contains(cleaned, "/") || strings.Contains(cleaned, "..") {
		return ""
	}
	switch cleaned {
	case "bootstrap.js", "usage.plugin.js", "ai-providers.plugin.js", "extensions.css":
		return cleaned
	default:
		return ""
	}
}
