package bclib

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"sort"
	"time"
)

type OpType int

const (
	Create OpType = iota
	Append
	Delete
)

type Operation struct {
	Op          OpType
	Data        []byte
	Filename    string
	MinerID     string
	OperationID string
	Timestamp   int64
}

func NewOperation(t OpType, mID string, fname string, data []byte) Operation {
	op := Operation{
		Op:        t,
		MinerID:   mID,
		Data:      data,
		Filename:  fname,
		Timestamp: time.Now().UnixNano(),
	}

	b, _ := MarshallOperation(op)
	hash := md5.Sum(b)
	op.OperationID = string(hash[:])

	return op
}

func FilterInvalidOperations(ops []Operation) []Operation {
	createFnames := make(map[string]struct{})
	opIDs := make(map[string]struct{})

	sort.Slice(ops, func(i, j int) bool {
		return ops[i].Timestamp < ops[j].Timestamp
	})

	validOps := make([]Operation, 0)
	for _, op := range ops {
		if op.Op == Create {
			_, ok := createFnames[op.Filename]
			if !ok {
				validOps = append(validOps, op)
				createFnames[op.Filename] = struct{}{}
			}
			continue
		}

		_, ok := opIDs[op.OperationID]
		if !ok {
			validOps = append(validOps, op)
			opIDs[op.OperationID] = struct{}{}
		}
	}

	return validOps
}

func ListOpsFromMap(ops map[string]Operation) []Operation {
	result := make([]Operation, 0, len(ops))

	for _, op := range ops {
		result = append(result, op)
	}

	return result
}

type DecodeOperationError string

func (e DecodeOperationError) Error() string {
	return fmt.Sprintf("Decoding Operation - [%s]", string(e))
}

func MarshallOperation(op Operation) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(op)
	return buf.Bytes(), err
}

func UnmarshallOperation(b []byte) (Operation, error) {
	var msg Operation
	bufDecoder := bytes.NewBuffer(b)
	decoder := gob.NewDecoder(bufDecoder)
	err := decoder.Decode(&msg)
	if err != nil {
		err = DecodeOperationError(err.Error())
	}
	return msg, err
}

func MarshallOperations(ops []Operation) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(ops)
	return buf.Bytes(), err
}
