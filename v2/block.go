package v2

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

type Block struct {
	Type int8
	Tag  int32
	Data []byte
}

func (block Block) WriteTo(w io.Writer) (rs int, err error) {
	bw := bufio.NewWriter(w)
	dataLen := 4 + 1 + 4
	if block.Data != nil {
		dataLen += len(block.Data)
	}
	err = binary.Write(bw, binary.BigEndian, int32(dataLen))
	if err != nil {
		return
	}
	err = binary.Write(bw, binary.BigEndian, int32(block.Tag))
	if err != nil {
		return
	}
	err = binary.Write(bw, binary.BigEndian, int8(block.Type))
	if err != nil {
		return
	}
	if block.Data != nil && len(block.Data) > 0 {
		n, err := bw.Write(block.Data)
		if err != nil {
			return n, err
		}
		if n < len(block.Data) {
			return n, io.ErrShortWrite
		}
		rs += n
	}
	if err == nil {
		bw.Flush()
	}
	return
}

type BlockWriter struct {
	closed     bool
	Tag        int32
	LastAccess time.Time
	Writer     io.WriteCloser
}

func (w BlockWriter) WriteBlock(tp int8, dat []byte) (n int, err error) {
	if w.closed {
		return 0, errors.New("writer closed")
	}
	block := Block{Data: dat, Tag: w.Tag, Type: tp}
	return block.WriteTo(w.Writer)
}

func (w BlockWriter) Write(p []byte) (n int, err error) {
	return w.WriteBlock(TYPE_DATA, p)
}

func (w BlockWriter) Close() error {
	w.closed = true
	return nil
}

func NewBlockWriter(w io.WriteCloser, tag int32) BlockWriter {
	return BlockWriter{Writer: w, closed: false, Tag: tag}
}

type BlockReader struct {
	Reader     io.Reader
	LastAccess time.Time
}

func (blockReader BlockReader) Read() (block *Block, err error) {
	block = &Block{}
	var blockSize int32 = 0
	err = binary.Read(blockReader.Reader, binary.BigEndian, &blockSize)
	if err != nil {
		return
	}
	err = binary.Read(blockReader.Reader, binary.BigEndian, &block.Tag)
	if err != nil {
		return
	}
	err = binary.Read(blockReader.Reader, binary.BigEndian, &block.Type)
	if err != nil {
		return
	}
	bodySize := blockSize - 9
	if bodySize > 0 {
		block.Data = make([]byte, bodySize)
		_, err = io.ReadFull(blockReader.Reader, block.Data)
		if err != nil {
			return nil, err
		}
	}
	return
}
