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

type TrackReader struct {
	ch     chan (int)
	reader io.ReadCloser
}

func NewTrackReader(ch chan (int), r io.ReadCloser) TrackReader {
	return TrackReader{ch: ch, reader: r}
}

func (r TrackReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.ch <- n
	return
}
func (r TrackReader) Close() error {
	return r.reader.Close()
}

type TrackWriter struct {
	ch     chan (int)
	writer io.WriteCloser
}

func NewTrackWriter(ch chan (int), w io.WriteCloser) TrackWriter {
	return TrackWriter{ch: ch, writer: w}
}

func (w TrackWriter) Write(p []byte) (n int, err error) {
	n, err = w.writer.Write(p)
	w.ch <- n
	return
}

func (w TrackWriter) Close() error {
	return w.writer.Close()
}
