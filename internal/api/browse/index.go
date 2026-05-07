package browse

import (
	"fmt"
	"net/http"
)

func (h *httpHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "browse: index (stub)")
}

func (h *httpHandler) entityHandler(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "browse: %s entity (stub)\n", kind)
	}
}
