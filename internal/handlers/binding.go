package handlers

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gin-gonic/gin"
)

// BindNestedOrFlat attempts to bind the request body to obj.
// It first checks if the body contains a nested object with the given key (e.g. {"project": {...}}).
// If so, it binds that nested object to obj.
// If not, or if the key is missing, it attempts to bind the entire body to obj (e.g. {...}).
// This helper is used to support both nested and flat JSON structures for compatibility.
func BindNestedOrFlat(c *gin.Context, key string, obj interface{}) error {
	var bodyBytes []byte
	if c.Request.Body != nil {
		bodyBytes, _ = io.ReadAll(c.Request.Body)
	}
	// Restore body for future binding or subsequent reads
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// 1. Try Nested Structure { "key": { ... } }
	var nestedMap map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &nestedMap); err == nil {
		if val, ok := nestedMap[key]; ok {
			// Found the key, try to unmarshal the content of that key into obj
			// We return the result of this unmarshal directly.
			// If the nested object is invalid for the target struct, we return that error.
			return json.Unmarshal(val, obj)
		}
	}

	// 2. Fallback to Flat Structure { ... }
	// If the nested key was not found or the body wasn't a JSON object, try flat binding.
	return json.Unmarshal(bodyBytes, obj)
}
