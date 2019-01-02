package mining

import (
	"bytes"
	"github.com/jlingohr/miner/bclib"
	"github.com/jlingohr/miner/settings"
	"testing"
	"time"
)

var genesisBlockHash = "83218ac34c1834c26781fe4bde918ee4"

func createSettings(MinerID string, PeerMinersAddrs []string, IncomingMinersAddr string, OutgoingMinersIP string, IncomingClientsAddr string) settings.Settings {
	s := settings.Settings{
		MinerID:             MinerID,
		PeerMinersAddrs:     PeerMinersAddrs,
		IncomingMinersAddr:  IncomingMinersAddr,
		OutgoingMinersIP:    OutgoingMinersIP,
		IncomingClientsAddr: IncomingClientsAddr,
		GenesisBlockHash:    genesisBlockHash,
	}
	return s
}

func TestMineSingleNoOpBlock(t *testing.T) {
	bc := bclib.NewBlockchain(createSettings("TestA", []string{}, "127.0.0.1:8080", "", ""))
	minedBlocks := make(chan bclib.Block)
	worker := NewWorker("TestA", 10, 10, bc, minedBlocks)
	defer worker.Stop()
	batchedOps := make(chan []bclib.Operation)
	go worker.StartMining(batchedOps, []byte(genesisBlockHash))
	//worker.ContinueMining <- struct{}{}

	block := <-minedBlocks
	if string(block.PreviousHash) != genesisBlockHash {
		t.Errorf("Expected previous hash %s, got %s", genesisBlockHash, string(block.PreviousHash))
	}
}

func TestMineMultipleNoOpBlocks(t *testing.T) {
	bc := bclib.NewBlockchain(createSettings("TestA", []string{}, "127.0.0.1:8080", "", ""))
	minedBlocks := make(chan bclib.Block)
	worker := NewWorker("TestA", 10, 10, bc, minedBlocks)
	defer worker.Stop()
	batchedOps := make(chan []bclib.Operation)
	go worker.StartMining(batchedOps, []byte(genesisBlockHash))
	//worker.ContinueMining <- struct{}{}

	blockA := <-minedBlocks
	if string(blockA.PreviousHash) != genesisBlockHash {
		t.Errorf("Expected previous hash %s, got %s", genesisBlockHash, string(blockA.PreviousHash))
	}
	bc.AddBlock(&blockA)
	//worker.ContinueMining <- struct{}{}
	go worker.StartMining(batchedOps, blockA.Hash)
	blockB := <-minedBlocks
	if bytes.Compare(blockA.Hash, blockB.PreviousHash) != 0 {
		t.Errorf("Expected previous hash %x, got %x", blockA.Hash, blockB.PreviousHash)
	}
}

func TestPreEmptNoOpBlock(t *testing.T) {
	bc := bclib.NewBlockchain(createSettings("TestA", []string{}, "127.0.0.1:8080", "", ""))
	minedBlocks := make(chan bclib.Block)
	worker := NewWorker("TestA", 20, 10, bc, minedBlocks)
	batchedOps := make(chan []bclib.Operation)
	go worker.StartMining(batchedOps, []byte(genesisBlockHash))
	worker.Stop()

	select {
	case b := <-minedBlocks:
		t.Errorf("Expected no new block. Got block with has %x", b.Hash)
	case <-time.After(1 * time.Second):
	}
}

func TestStartMiningOpBlock(t *testing.T) {
	bc := bclib.NewBlockchain(createSettings("TestA", []string{}, "127.0.0.1:8080", "", ""))
	minedBlocks := make(chan bclib.Block)
	worker := NewWorker("TestA", 20, 10, bc, minedBlocks)
	batchedOps := make(chan []bclib.Operation)
	go worker.StartMining(batchedOps, []byte(genesisBlockHash))

	operation := bclib.NewOperation(bclib.Create, "TestA", "TestFile", []byte("test data"))
	batchedOps <- []bclib.Operation{operation}

	select {
	case b := <-minedBlocks:
		if len(b.Operations) != 1 {
			t.Errorf("Expected a single operation, got %d", len(b.Operations))
		}
	case <-time.After(5 * time.Second):
		t.Error("Expected to mind a new block")
	}
}
