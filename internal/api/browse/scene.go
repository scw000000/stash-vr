package browse

import (
	"fmt"
	"net/http"
)

func (h *httpHandler) sceneDetailHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "browse: scene detail (stub)")
}
