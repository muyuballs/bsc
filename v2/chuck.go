package v2

import (
	"encoding/binary"
	"io"
	"time"
)

type Chuck struct {
	len  []byte
	Tag  byte
	Data []byte
}

func NewChuck(data []byte, tag byte) Chuck {
	ch := Chuck{Data: data, Tag: tag, len: make([]byte, 2)}
	binary.BigEndian.PutUint16(ch.len, uint16(len(data)+3))
	return ch
}

func (ch Chuck) Len() int {
	return int(binary.BigEndian.Uint16(ch.len))
}

func (ch Chuck) WriteTo(w io.Writer) (rs int, err error) {
	n, err := w.Write(ch.len)
	if n < 2 {
		err = io.ErrShortWrite
		return
	}
	rs += n
	n, err = w.Write([]byte{ch.Tag})
	if n < 1 {
		err = io.ErrShortWrite
		return
	}
	rs += n
	n, err = w.Write(ch.Data)
	if n < len(ch.Data) {
		err = io.ErrShortWrite
		return
	}
	rs += n
	return
}

type ChunckWriter struct {
	Tag        byte
	LastAccess time.Time
	Writer     io.WriteCloser
}

func (cw ChunckWriter) Write(p []byte) (n int, err error) {
	ch := NewChuck(p, cw.Tag)
	return ch.WriteTo(cw.Writer)
}

type ChunckReader struct {
	Reader     io.Reader
	LastAccess time.Time
}

func (cr ChunckReader) Read() (ch *Chuck, err error) {
	ch = &Chuck{len: make([]byte, 2)}
	_, err = io.ReadFull(cr.Reader, ch.len)
	if err != nil {
		return nil, err
	}
	tag := make([]byte, 1)
	_, err = io.ReadFull(cr.Reader, tag)
	if err != nil {
		return nil, err
	}
	ch.Tag = tag[0]
	ch.Data = make([]byte, ch.Len())
	_, err = io.ReadFull(cr.Reader, ch.Data)
	if err != nil {
		return nil, err
	}
	return ch, nil
}
