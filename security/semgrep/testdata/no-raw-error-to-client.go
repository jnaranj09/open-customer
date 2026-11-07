package fixtures

import (
	"fmt"
	"net/http"
)

func writesTheRawError(w http.ResponseWriter, r *http.Request, err error) {
	// ruleid: no-raw-error-to-client
	http.Error(w, err.Error(), http.StatusBadRequest)
	// ruleid: no-raw-error-to-client
	http.Error(w, fmt.Sprintf("failed: %v", err), http.StatusInternalServerError)
	// ruleid: no-raw-error-to-client
	http.Error(w, "failed: "+err.Error(), http.StatusInternalServerError)
	// ruleid: no-raw-error-to-client
	w.Write([]byte(err.Error()))
	// ruleid: no-raw-error-to-client
	fmt.Fprintf(w, "failed: %s", err)
	// ruleid: no-raw-error-to-client
	fmt.Fprint(w, err)
}

func writesAFixedText(w http.ResponseWriter, r *http.Request, err error) {
	// ok: no-raw-error-to-client
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	// ok: no-raw-error-to-client
	http.Error(w, "not found", http.StatusNotFound)
	// ok: no-raw-error-to-client
	w.Write([]byte("ok"))
	// ok: no-raw-error-to-client
	fmt.Println(err.Error())
}
