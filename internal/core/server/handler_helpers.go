package server

import (
	"encoding/json"
	"net/http"
)

// decodeJSONBody decodes a JSON request body into dst.
// It rejects unknown fields to catch client typos and malformed requests early.
func decodeJSONBody(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
