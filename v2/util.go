package v2

import (
	"io"
	"net/http"
)

func ServiceUnavailable(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	if r.Body != nil {
		r.Body.Close()
	}
}
func ReadByte(r io.Reader) (rel byte, err error) {
	buf := make([]byte, 1)
	_, err = io.ReadFull(r, buf)
	if err == nil {
		rel = buf[0]
	}
	return
}
