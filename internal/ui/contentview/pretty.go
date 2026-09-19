package contentview

import (
	"bytes"
	"encoding/json"
)

// PrettifyJSON format JSON with indent. Return ASIS if not JSON
func PrettifyJSON(b []byte) []byte {
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		return b
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return b
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}
