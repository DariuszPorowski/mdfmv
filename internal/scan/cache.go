package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schemaCache caches compiled schemas by a stable identity key.
type schemaCache struct {
	mu       sync.Mutex
	compiled map[string]*jsonschema.Schema
}

func (sc *schemaCache) get(key string) (*jsonschema.Schema, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.compiled == nil {
		return nil, false
	}

	s, ok := sc.compiled[key]

	return s, ok
}

func (sc *schemaCache) set(key string, s *jsonschema.Schema) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.compiled == nil {
		sc.compiled = make(map[string]*jsonschema.Schema)
	}

	sc.compiled[key] = s
}

// inlineSchemaKey generates a stable key for inline schema maps.
func inlineSchemaKey(m map[string]any) string {
	b, err := json.Marshal(m)
	if err != nil {
		// On marshal error, disable caching for this inline schema by returning empty key
		return ""
	}

	sum := sha256.Sum256(b)

	return "inline:" + hex.EncodeToString(sum[:])
}
