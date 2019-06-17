package mining

import (
	"github.com/jlingohr/rfs-miner/bclib"
	"log"
)

type Worker struct {
	MinerID         string
	PowPerNoOpBlock uint8
	PowPerOpBlock   uint8

	bcManager bclib.RFSLedger

	minedBlock chan<- bclib.Block
	stop       chan struct{}
}

func NewWorker(id string, powPerNoOpBlock uint8, powPerOpBlock uint8, ledger bclib.RFSLedger, minedBlock chan<- bclib.Block) *Worker {
	w := &Worker{
		MinerID:         id,
		bcManager:       ledger,
		PowPerNoOpBlock: powPerNoOpBlock,
		PowPerOpBlock:   powPerOpBlock,
		minedBlock:      minedBlock,
		stop:            make(chan struct{}),
	}

	return w
}

func (w *Worker) StartMining(batchedOperations <-chan []bclib.Operation) {
	stopNoOp := make(chan struct{})
	doneNoOp := make(chan struct{})
	noOpPreviousHash := w.bcManager.GetLastHash()
	go w.mineNoOp(stopNoOp, doneNoOp, noOpPreviousHash)

	powPerOpBlock := w.PowPerOpBlock

	select {
	// todo remove w.stop channel??
	case <-w.stop:
		close(stopNoOp)
		return
	case <-doneNoOp:
		return
	case ops := <-batchedOperations:
		filteredOperations := bclib.FilterInvalidOperations(ops)
		prevHash := w.bcManager.GetLastHash()
		bestOps := make([]bclib.Operation, 0)

		lastBlocks := w.bcManager.GetLastBlocks()

		for _, block := range lastBlocks {
			validatedOperations, _ := w.bcManager.ValidateOperations(filteredOperations, prevHash)
			if len(validatedOperations) >= len(bestOps) {
				bestOps = validatedOperations
				prevHash = block.Hash
			}
		}

		if len(bestOps) > 0 {
			close(stopNoOp)
			block := bclib.NewBlock(bestOps, prevHash, w.MinerID, int(powPerOpBlock), nil)
			w.minedBlock <- *block
			return
		}
	}
}

func (w *Worker) Stop() {
	close(w.stop)
}

func (w *Worker) mineNoOp(stop chan struct{}, done chan struct{}, previousHash []byte) {
	powPerNoOpBlock := w.PowPerNoOpBlock
	minerID := w.MinerID

	log.Printf("Mining previous hash %x", previousHash[:])
	block := bclib.NewBlock([]bclib.Operation{}, previousHash, minerID, int(powPerNoOpBlock), stop)
	close(done)
	w.minedBlock <- *block
	//if block.Nonce >= 0 {
	//	log.Printf("Mined %x for previous hash %x", block.Hash[:], block.PreviousHash[:])
	//
	//} else {
	//	log.Printf("Cancelled mining previous hash %x", block.PreviousHash[:])
	//	return
	//}
}
