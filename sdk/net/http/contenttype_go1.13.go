// +build go1.13,!go1.14

package http

import (
	"net/http"
	"strings"
)

// shouldRecordBodyOfContentType checks if the body is meant
// to be recorded based on the content-type and the fact that body is
// not streamed.
func shouldRecordBodyOfContentType(h http.Header) bool {
	for _, contentTypeAllowed := range contentTypeAllowList {
		lContentType := strings.ToLower(h.Get("content-type"))
		// userland code can set joint headers directly instead of adding
		// them for example
		//
		// ```
		//   header.Set("content-type", "application/json; charset=utf-8")
		// ```
		//
		// instead of
		//
		// ```
		//    header.Add("content-type", "application/json")
		//    header.Add("content-type", "charset=utf-8")
		// ```
		// hence we need to inspect it by using contains.
		if strings.Contains(lContentType, contentTypeAllowed) {
			return true
		}
	}
	return false
}