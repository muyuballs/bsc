package v2

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"
)

const (
	BL_TYPE_DATA  = 0
	BL_TYPE_OPEN  = 1
	BL_TYPE_CLOSE = 2
)

type Block struct {
	Type byte
	Tag  byte
	Data []byte
}

func (block Block) WriteTo(w *net.TCPConn) (rs int, err error) {
	bw := bufio.NewWriter(w)
	lenBuf := make([]byte, 6)
	dataLen := 6
	if block.Data != nil {
		dataLen += len(block.Data)
	}
	binary.BigEndian.PutUint32(lenBuf, uint32(dataLen))
	lenBuf[4] = block.Tag
	lenBuf[5] = block.Type
	n, err := bw.Write(lenBuf)
	if n < len(lenBuf) {
		err = io.ErrShortWrite
		return
	}
	if err != nil {
		return
	}
	if block.Data != nil && len(block.Data) > 0 {
		n, err = bw.Write(block.Data)
		if n < len(block.Data) {
			err = io.ErrShortWrite
			return
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
	Tag        byte
	LastAccess time.Time
	Writer     *net.TCPConn
}

func (blockWriter BlockWriter) WriteBlock(tp int, dat []byte) (n int, err error) {
	if blockWriter.closed {
		return 0, errors.New("writer closed")
	}
	block := Block{Data: dat, Tag: blockWriter.Tag, Type: byte(tp)}
	return block.WriteTo(blockWriter.Writer)
}

func (blockWriter BlockWriter) Write(p []byte) (n int, err error) {
	return blockWriter.WriteBlock(BL_TYPE_DATA, p)
}

func (blockWriter BlockWriter) Close() error {
	blockWriter.closed = true
	return nil
}

func NewBlockWriter(w *net.TCPConn, tag byte) BlockWriter {
	return BlockWriter{Writer: w, closed: false, Tag: tag}
}

type BlockReader struct {
	Reader     io.Reader
	LastAccess time.Time
}

func (blockReader BlockReader) Read() (block *Block, err error) {
	blockReader.LastAccess = time.Now()
	headBuf := make([]byte, 6)
	_, err = io.ReadFull(blockReader.Reader, headBuf)
	if err != nil {
		return nil, err
	}
	block = &Block{Tag: headBuf[4], Type: headBuf[5]}
	bodyLen := binary.BigEndian.Uint32(headBuf) - 6
	if bodyLen > 0 {
		block.Data = make([]byte, bodyLen)
		_, err = io.ReadFull(blockReader.Reader, block.Data)
		if err != nil {
			return nil, err
		}
	}
	return
}
