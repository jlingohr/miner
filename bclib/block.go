package bclib

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

type Header struct {
	Timestamp    int64
	Operations   []Operation
	PreviousHash []byte
}

type Block struct {
	Header
	MinerID string
	Hash    []byte
	Nonce   int
}

func NewBlock(ops []Operation, prevHash []byte, mID string, difficulty int, stop chan struct{}) *Block {
	block := &Block{
		Header: Header{
			Timestamp:    time.Now().Unix(),
			Operations:   ops,
			PreviousHash: prevHash,
		},
		MinerID: mID,
		Hash:    []byte{},
		Nonce:   0,
	}

	pow := NewProofOfWork(block, difficulty)
	nonce, hash := pow.Run(stop)
	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func NewGenesisBlock(genBlockHash string) *Block {
	block := &Block{
		Header: Header{
			Timestamp:    time.Now().Unix(),
			Operations:   []Operation{},
			PreviousHash: []byte{},
		},
		Hash:  []byte(genBlockHash),
		Nonce: 0,
	}

	return block
}

type DecodeBlockError string

func (e DecodeBlockError) Error() string {
	return fmt.Sprintf("Decoding Block - [%s]", string(e))
}

func MarshallBlock(op Block) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(op)
	return buf.Bytes(), err
}

func UnmarshallBlock(b []byte) (Block, error) {
	var msg Block
	bufDecoder := bytes.NewBuffer(b)
	decoder := gob.NewDecoder(bufDecoder)
	err := decoder.Decode(&msg)
	if err != nil {
		return Block{}, DecodeBlockError(err.Error())
	}

	if msg.Operations == nil {
		msg.Operations = make([]Operation, 0)
	}

	return msg, nil
}

func MarshallBlocks(blocks []Block) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(blocks)

	return buf.Bytes(), err
}

func UnmarshallBlocks(data []byte) ([]Block, error) {
	blocks := make([]Block, 0)

	bufDecoder := bytes.NewBuffer(data)
	decoder := gob.NewDecoder(bufDecoder)
	err := decoder.Decode(&blocks)

	return blocks, err
}
