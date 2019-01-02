package bclib

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"log"
	"math"
	"math/big"
)

var (
	maxNonce = math.MaxInt64
)

type ProofOfWork struct {
	block      *Block
	difficulty uint
}

func NewProofOfWork(b *Block, difficulty int) *ProofOfWork {
	pow := &ProofOfWork{
		block:      b,
		difficulty: uint(difficulty * 4),
	}
	return pow
}

func (pow *ProofOfWork) prepareData(nonce int) []byte {
	operations := make([]byte, 0)

	if pow.block.Operations != nil && len(pow.block.Operations) > 0 {
		operations, _ = MarshallOperations(pow.block.Operations)
	}

	data := bytes.Join(
		[][]byte{
			pow.block.PreviousHash,
			operations,
			[]byte(pow.block.MinerID),
			IntToHex(pow.block.Timestamp),
			IntToHex(int64(pow.difficulty)),
			IntToHex(int64(nonce)),
		},
		[]byte{},
	)
	return data
}

func (pow *ProofOfWork) Run(stop chan struct{}) (int, []byte) {
	var hashInt big.Int
	var rshHashInt big.Int
	var hash [16]byte
	nonce := 0

	for nonce < maxNonce {
		select {
		case <-stop:
			return -1, []byte{}
		default:
			data := pow.prepareData(nonce)
			hash = md5.Sum(data)

			hashInt.SetBytes(hash[:])
			rshHashInt.SetBytes(hash[:])

			rshHashInt.Rsh(&rshHashInt, pow.difficulty)
			rshHashInt.Lsh(&rshHashInt, pow.difficulty)

			if hashInt.Cmp(&rshHashInt) == 0 {
				return nonce, hash[:]
			} else {
				nonce++
			}
		}

	}
	return nonce, hash[:]
}

func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int
	var rshHashInt big.Int

	data := pow.prepareData(pow.block.Nonce)
	hash := md5.Sum(data)
	hashInt.SetBytes(hash[:])
	rshHashInt.SetBytes(hash[:])

	rshHashInt.Rsh(&rshHashInt, pow.difficulty)
	rshHashInt.Lsh(&rshHashInt, pow.difficulty)

	isValid := hashInt.Cmp(&rshHashInt) == 0
	return isValid
}

func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}
