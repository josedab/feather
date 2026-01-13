package server

import (
	"encoding/json"
	"net/http"
)

// decodeJSONBody decodes a JSON request body into dst.
func decodeJSONBody(r *http.Request, dst interface{}) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
