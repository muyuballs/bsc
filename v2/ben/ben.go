package ben

import (
	"encoding/binary"
	"errors"
	"io"
)

func ReadLDString(r io.Reader) (rel string, err error) {
	length := 0
	err = binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return
	}
	if length < 0 {
		return "", errors.New("length must >= 0")
	}
	if length < 1 {
		return "", nil
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return
	}
	return string(buf), nil
}

func WriteLDString(w io.Writer, str string) (err error) {
	err = binary.Write(w, binary.BigEndian, len(str))
	if err == nil {
		_, err = w.Write([]byte(str))
	}
	return
}
