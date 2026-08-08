package store

import (
	"path/filepath"
	"strconv"
	"strings"

	"go-drive/server/internal/model"
)

// StoreRootPrefix returns the store's configured root prefix ("" when none).
// Read directly from the store's config JSON since BuildStorage does not
// surface it on the storage.Config struct.
func StoreRootPrefix(s *model.Store) string {
	if v, ok := s.Config["rootPrefix"].(string); ok {
		return strings.Trim(v, "/")
	}
	return ""
}

// DisplayPathFromKey strips the store's root prefix from an external key,
// returning the workspace display path (e.g. "docs/reports/q1.pdf"). It strips
// both the rootPrefix itself and any platform org prefix ("<rootPrefix>/<org>/").
func DisplayPathFromKey(key, rootPrefix string) string {
	p := key
	if rootPrefix != "" {
		if p == rootPrefix {
			return ""
		}
		if strings.HasPrefix(p, rootPrefix+"/") {
			p = strings.TrimPrefix(p, rootPrefix+"/")
		} else {
			return "" // not under this store's prefix
		}
	}
	return strings.Trim(p, "/")
}

// DedupName returns name, or name (1), name (2), ... when name is already taken
// among the given existing names. The existing map is updated in place.
func DedupName(name string, existing map[string]bool) string {
	if !existing[name] {
		existing[name] = true
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := base + " (" + strconv.Itoa(i) + ")" + ext
		if !existing[candidate] {
			existing[candidate] = true
			return candidate
		}
	}
}
