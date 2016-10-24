package v2

import (
	"fmt"
	"io"
	"net/http"
)

func ServiceUnavailable(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	if r.Body != nil {
		r.Body.Close()
	}
}
func ServerError(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
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

func FormatSize(val int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
		TB = 1 << 40
		PB = 1 << 50
	)
	if val > PB {
		return fmt.Sprintf("%vPB", val/PB)
	}
	if val > TB {
		return fmt.Sprintf("%vTB", val/TB)
	}
	if val > GB {
		return fmt.Sprintf("%vGB", val/GB)
	}
	if val > MB {
		return fmt.Sprintf("%vMB", val/MB)
	}
	if val > KB {
		return fmt.Sprintf("%vKB", val/KB)
	}
	return fmt.Sprintf("%dB", val)
}
