package ben

import (
	"encoding/binary"
	"errors"
	"io"
)

func ReadString(r io.Reader) (rel string, err error) {
	var length int32 = 0
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

func WriteString(w io.Writer, str string) (err error) {
	err = binary.Write(w, binary.BigEndian, int32(len(str)))
	if err == nil {
		_, err = w.Write([]byte(str))
	}
	return
}
func ReadUInt(r io.Reader) (rel uint, err error) {
	var t int32 = 0
	err = binary.Read(r, binary.BigEndian, &t)
	if err != nil {
		return
	}
	var ui32 = 0
	err = binary.Read(r, binary.BigEndian, &ui32)
	if err == nil {
		rel = uint(ui32)
	}
	return
}
func WriteUInt(w io.Writer, val uint32) (err error) {
	err = binary.Write(w, binary.BigEndian, 4)
	if err == nil {
		err = binary.Write(w, binary.BigEndian, val)
	}
	return
}
