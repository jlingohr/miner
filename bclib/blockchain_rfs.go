package bclib

type RFSLedger interface {
	GetLastHash() []byte
	GetLastBlocks() []*Block
	ValidateOperations(filteredOperations []Operation, prevHash []byte) ([]Operation, bool)
	ListFiles() []string
	GetFile(fname string) (FileOps, bool)
	NumRecs(fname string) (int, bool)
	CheckOpConfirmed(op Operation) (confirmed, exists, otherMiner bool)
}

type FileOps struct {
	Fname      string
	Operations []Operation
	Deleted    bool
}

// For each miner involved in the operations of the block, make sure they have positive balance
// TODO: where do we check that there are no conflicting operations within the block?
// TODO: like two operations with the same id for example, or two operation for file creation with the same name
// TODO: check append for a file that does not exist???
// TODO: handle create file -> delete file -> create file with the same name workflow
// Assumes that operations are non-conflicting
func (bc *Blockchain) ValidateOperations(operations []Operation, previousHash []byte) ([]Operation, bool) {
	if len(operations) == 0 {
		return []Operation{}, true
	}

	minersBalance := make(map[string]int, len(operations))

	// Map IDs to Operations, conflicting operations will be removed from this map
	opIds := make(map[string]Operation, len(operations))
	// Map "create file" operations filename to operation to check for duplicate "create file" ops
	files := make(map[string]Operation)

	for _, op := range operations {
		if op.Op == Create {
			files[op.Filename] = op
		}

		minersBalance[op.MinerID] = 0
		opIds[op.OperationID] = op
	}

	// start backtracking from the parent block of the block that we are validating
	node, ok := bc.m[string(previousHash[:])]

	if !ok {
		return []Operation{}, false
	}

	// backtrack to the genesis block
	for node != nil {
		// if a block was added by a miner associated with operations in the input block
		// increase their balance by 1
		if balance, ok := minersBalance[node.block.MinerID]; ok {
			if len(node.block.Operations) > 0 {
				minersBalance[node.block.MinerID] = balance + int(bc.config.MinedCoinsPerOpBlock)
			} else {
				minersBalance[node.block.MinerID] = balance + int(bc.config.MinedCoinsPerNoOpBlock)
			}
		}

		// if a miner associated with operations in the input block submitted an operation
		// in other blocks, decrease their balance by 1 for each operation
		for _, op := range node.block.Operations {
			if balance, ok := minersBalance[op.MinerID]; ok {
				cost := 1

				if op.Op == Create {
					cost = int(bc.config.NumCoinsPerFileCreate)

					if conflictingOp, ok := files[op.Filename]; ok {
						// Duplicate "create file" operation, delete from the valid operations map
						delete(opIds, conflictingOp.OperationID)
					}
				}

				minersBalance[op.MinerID] = balance - cost
			}

			if _, ok := opIds[op.OperationID]; ok {
				// Operation with the same id encountered, delete from the valid operations map
				delete(opIds, op.OperationID)
			}
		}

		node = node.parent
	}

	conflictOpIds := make([]string, 0)

	// For the remaining valid operations, make sure the miners have enough coins
	for _, op := range opIds {
		minerBalance, ok := minersBalance[op.MinerID]

		if !ok {
			return []Operation{}, false
		}

		cost := 1

		if op.Op == Create {
			cost = int(bc.config.NumCoinsPerFileCreate)
		}

		if (minerBalance - cost) < 0 {
			conflictOpIds = append(conflictOpIds, op.OperationID)
		} else {
			minerBalance -= cost
		}

		minersBalance[op.MinerID] = minerBalance
	}

	for _, id := range conflictOpIds {
		delete(opIds, id)
	}

	validOps := make([]Operation, 0)

	for _, op := range opIds {
		validOps = append(validOps, op)
	}

	return validOps, len(operations) == len(validOps)
}

func (bc *Blockchain) ListFiles() []string {
	filenames := make(map[string]struct{})

	node, ok := bc.getLastConfirmedNode(int(bc.config.ConfirmsPerFileCreate))

	if !ok {
		return []string{}
	}

	for node != nil {
		for _, op := range node.block.Operations {
			filenames[op.Filename] = struct{}{}
		}

		node = node.parent
	}

	res := make([]string, 0, len(filenames))

	for key := range filenames {
		res = append(res, key)
	}

	return res
}

// Returns false if the file with fname does not exist
// TODO: remove this and use GetFile instead?
func (bc *Blockchain) NumRecs(fname string) (int, bool) {
	node, ok := bc.getLastConfirmedNode(int(bc.config.ConfirmsPerFileAppend))
	numRecs := 0
	// FIXME: redundant?
	seenFilename := false

	if !ok {
		return -1, false
	}

	for node != nil {

		for _, op := range node.block.Operations {
			if op.Filename == fname {
				seenFilename = true

				if op.Op == Append {
					numRecs += 1
				}
			}
		}

		node = node.parent
	}

	return numRecs, seenFilename
}

// Returns false if the file with fname does not exist
func (bc *Blockchain) GetFile(fname string) (FileOps, bool) {
	node, ok := bc.getLastConfirmedNode(int(bc.config.ConfirmsPerFileAppend))

	if !ok {
		return FileOps{}, false
	}

	operations := make([]Operation, 0)
	seenFilename := false // if file exists but has zero recs
	deleted := false

	for node != nil {
		for _, op := range node.block.Operations {
			if op.Filename == fname {
				seenFilename = true

				if op.Op == Append {
					operations = append(operations, op)
				}

				if op.Op == Delete {
					// TODO: can we list the number of recs of deleted files?
					// TODO: return false here right away meaning that the file does not exist?
					deleted = true
				}
			}
		}

		node = node.parent
	}

	// reverse the order of operations
	for i, j := 0, len(operations)-1; i < j; i, j = i+1, j-1 {
		operations[i], operations[j] = operations[j], operations[i]
	}

	fileOps := FileOps{
		Operations: operations,
		Deleted:    deleted,
		Fname:      fname,
	}

	return fileOps, seenFilename
}

func (bc *Blockchain) CheckOpConfirmed(op Operation) (confirmed, exists, otherMiner bool) {
	node := bc.getLastNodes()[0]
	numConfBlocks := bc.config.ConfirmsPerFileCreate

	if op.Op == Append {
		numConfBlocks = bc.config.ConfirmsPerFileAppend
	}

	i := 0

	for node != nil {

		for _, blockOp := range node.block.Operations {
			if op.Op == Create && op.Filename == blockOp.Filename && op.MinerID != blockOp.MinerID {
				otherMiner = true
			}

			if blockOp.OperationID == op.OperationID {
				if i > int(numConfBlocks) {
					confirmed = true
				}
				exists = true
				return
			}
		}

		i = i + 1
		node = node.parent
	}

	return
}
