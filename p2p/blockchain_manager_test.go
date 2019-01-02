package p2p

import (
	"bytes"
	"github.com/jlingohr/miner/bclib"
	"github.com/jlingohr/miner/settings"
	"log"
	"sync"
	"testing"
	"time"
)

var genesisBlockHash = "83218ac34c1834c26781fe4bde918ee4"

func createSettings(MinerID string, PeerMinersAddrs []string, IncomingMinersAddr string, OutgoingMinersIP string, IncomingClientsAddr string) settings.Settings {
	s := settings.Settings{
		MinerID:               MinerID,
		PeerMinersAddrs:       PeerMinersAddrs,
		IncomingMinersAddr:    IncomingMinersAddr,
		OutgoingMinersIP:      OutgoingMinersIP,
		IncomingClientsAddr:   IncomingClientsAddr,
		GenesisBlockHash:      "83218ac34c1834c26781fe4bde918ee4",
		ConfirmsPerFileAppend: 1,
		ConfirmsPerFileCreate: 1,
	}
	return s
}

func TestSynchronization(t *testing.T) {
	bc1 := bclib.NewBlockchain(createSettings("minerB", []string{}, "127.0.0.1:8080", "", ""))
	bc2 := bclib.NewBlockchain(createSettings("minerA", []string{}, "", "", ""))
	bc2.AddBlock(bclib.NewBlock(make([]bclib.Operation, 0), []byte(genesisBlockHash), "minerA", 1, nil))

	manager := NewChainManager(bc1, createSettings("minerB", []string{}, "127.0.0.1:8080", "127.0.0.1:8081", ""))
	manager.newBlockAdded = make(chan struct{})

	s := SynchronizeBlockchain{PeerBlocks: bc2.GetBlocks(), done: make(chan struct{}), err: make(chan error)}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		manager.syncBlockchain <- s
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		<-s.done
		wg.Done()
	}()

	wg.Wait()
}

// Test adding a single block that we mine. We should expect
// 1) receiving from channels:
// - manager.floodBlock
// - manager.continueMining
// 2) manager.isMining == false
func TestAddOwnAndFloodSingleBlock(t *testing.T) {
	bc1 := bclib.NewBlockchain(createSettings("minerB", []string{}, "127.0.0.1:8080", "", ""))
	newBlock := bclib.NewBlock(make([]bclib.Operation, 0), []byte(genesisBlockHash), "minerB", 1, nil)

	manager := NewChainManager(bc1, createSettings("minerB", []string{}, "127.0.0.1:8080", "127.0.0.1:8081", ""))
	manager.newBlockAdded = make(chan struct{})

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		manager.addBlock <- *newBlock
		wg.Done()
	}()

	go func() {
		<- manager.newBlockAdded
		return
	}()

	wg.Add(1)
	go func() {
		for {
			select {
			case floodRequest := <-manager.floodBlock:
				block := floodRequest.Block
				if bytes.Compare(newBlock.Hash, block.Hash) != 0 {
					t.Errorf("Expected hash %x, got %x", newBlock.Hash, block.Hash)
				}
				if string(newBlock.PreviousHash) != genesisBlockHash {
					t.Errorf("Expected previous hash %x, got %x", []byte(genesisBlockHash), newBlock.PreviousHash)
				}
				//continue
			case <-manager.continueMining:
				wg.Done()
				return

			}
		}
	}()

	wg.Wait()

	//if manager.isMining {
	//	t.Errorf("Expect ChainManager.isMining == false")
	//}

	if bc1.GetSize() != 2 {
		t.Errorf("Expected blockchain with 2 blocks, got %d", bc1.GetSize())
	}
}

// Test adding a single block that we dont' mine. We should expect
// 1) receiving from channels:
// - manager.floodBlock
// 2) manager.isMining == true
func TestAddOtherAndFloodSingleBlock(t *testing.T) {
	bc1 := bclib.NewBlockchain(createSettings("minerB", []string{}, "127.0.0.1:8080", "", ""))
	newBlock := bclib.NewBlock(make([]bclib.Operation, 0), []byte(genesisBlockHash), "minerA", 1, nil)

	manager := NewChainManager(bc1, createSettings("minerB", []string{}, "127.0.0.1:8080", "127.0.0.1:8081", ""))
	//manager.isMining = true
	manager.newBlockAdded = make(chan struct{})

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		manager.addBlock <- *newBlock
		wg.Done()
	}()

	go func() {
		<- manager.newBlockAdded
		return
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case floodRequest := <-manager.floodBlock:
				block := floodRequest.Block
				if bytes.Compare(newBlock.Hash, block.Hash) != 0 {
					t.Errorf("Expected hash %x, got %x", newBlock.Hash, block.Hash)
				}

				if string(newBlock.PreviousHash) != genesisBlockHash {
					t.Errorf("Expected previous hash %x, got %x", []byte(genesisBlockHash), newBlock.PreviousHash)
				}
				//continue
			case <-manager.continueMining:
				t.Errorf("Was signalled to continue mining, but do not expect to receive on this channel")
				return
			case <-time.After(3 * time.Second):
				return
			}
		}

		return
	}()

	wg.Wait()

	//if !manager.isMining {
	//	t.Errorf("Expect ChainManager.isMining == false")
	//}

	if bc1.GetSize() != 2 {
		t.Errorf("Expected blockchain with 2 blocks, got %d", bc1.GetSize())
	}
}

// Test receiving a flooded block we can add that we don't mine. We should expect
// 1) receiving from channels:
// - manager.floodBlock
// 2) manager.isMining == true
func TestFloodingBlockCanAdd(t *testing.T) {
	bc1 := bclib.NewBlockchain(config)
	manager := NewChainManager(bc1, createSettings(minerID, make([]string, 0), minerPeerAddress, outgoingMinersIP, minerClientAddress))
	manager.newBlockAdded = make(chan struct{})

	go func() {
		<- manager.newBlockAdded
		return
	}()

	serverA, err := NewMinerServer(configA)
	if err != nil {
		t.Fatal(err)
	}
	serverA.startListening()

	err = serverA.manager.blockchain.AddBlock(blockA)
	if err != nil {
		t.Fatal(err)
	}

	peerBlocks := serverA.manager.blockchain.GetSize()
	if peerBlocks != 2 {
		t.Fatalf("Expect peer to have 2 blocks, got %d", peerBlocks)
	}

	go func() {
		<-time.After(1 * time.Second)
		manager.receiveFloodedBlock <- NewFloodBlockReq{floodedReqA, make(chan struct{})}
	}()

	<-time.After(5 * time.Second)
	numBlocks := bc1.GetSize()
	if numBlocks != 2 {
		t.Errorf("Expected %d blocks in blockchain, got %d", 2, numBlocks)
	}

	serverA.shutdown()
}

//Test that if we are flooded a block for which we do not have the previous hash that we can
//download the missing block
func TestCanDownloadSingleBlock(t *testing.T) {
	bc1 := bclib.NewBlockchain(config)
	manager := NewChainManager(bc1, createSettings(minerID, make([]string, 0), minerPeerAddress, outgoingMinersIP, minerClientAddress))
	manager.newBlockAdded = make(chan struct{})

	go func() {
		<- manager.newBlockAdded
		return
	}()

	serverB, err := NewMinerServer(configB)
	if err != nil {
		t.Fatal(err)
	}
	serverB.startListening()

	err = serverB.manager.blockchain.AddBlock(blockA)
	if err != nil {
		t.Fatal(err)
	}
	err = serverB.manager.blockchain.AddBlock(blockB)
	if err != nil {
		t.Fatal(err)
	}

	peerBlocks := serverB.manager.blockchain.GetSize()
	if peerBlocks != 3 {
		t.Fatalf("Expect peer to have 3 blocks, got %d", peerBlocks)
	}

	go func() {
		<-time.After(1 * time.Second)
		manager.downloader.floodedBlock <- NewFloodBlockReq{FloodBlockReq{FromMinerId: minerB, Block: *blockA, FromMinerAddress: addressB}, make(chan struct{})}
		<-time.After(1 * time.Second)
		manager.receiveFloodedBlock <- NewFloodBlockReq{FloodBlockReq{FromMinerId: minerB, Block: *blockB, FromMinerAddress: addressB}, make(chan struct{})}
	}()

	go func() {
		for req := range manager.floodBlock {
			log.Printf("Test draining block %x", req.Block.Hash)
			hash := string(req.Block.Hash)
			if hash != string(blockA.Hash) && hash != string(blockB.Hash) {
				t.Fatalf("Expected to flood block with hash %x or %x, but flooding %x",
					blockA.Hash, blockB.Hash, req.Block.Hash)
				return
			}

			if req.FromMinerId != minerID {
				t.Fatalf("Expected to flood from miner %s, but got %s", minerID, req.FromMinerId)
				return
			}

			if len(req.ignorePeers) != 1 {
				t.Fatalf("Expected to ignore 1 peer, got %d", len(req.ignorePeers))
				return
			}
		}
	}()

	<-time.After(5 * time.Second)
	numBlocks := bc1.GetSize()
	if numBlocks != 3 {
		t.Errorf("Expected %d blocks in blockchain, got %d", 3, numBlocks)
	}
}

func TestResolvingSimpeDownloadChain(t *testing.T) {
	bc1 := bclib.NewBlockchain(config)
	manager := NewChainManager(bc1, createSettings(minerID, make([]string, 0), minerPeerAddress, outgoingMinersIP, minerClientAddress))
	manager.newBlockAdded = make(chan struct{})

	done := make(chan struct{})

	go func() {
		for {
			select {
			case <- done:
				return
			case <- manager.newBlockAdded:
			}
		}

	}()

	serverC, err := NewMinerServer(configC)
	if err != nil {
		t.Fatal(err)
	}
	serverC.startListening()

	err = serverC.manager.blockchain.AddBlock(blockA)
	if err != nil {
		t.Fatal(err)
	}
	err = serverC.manager.blockchain.AddBlock(blockB)
	if err != nil {
		t.Fatal(err)
	}

	err = serverC.manager.blockchain.AddBlock(blockC)
	if err != nil {
		t.Fatal(err)
	}

	peerBlocks := serverC.manager.blockchain.GetSize()
	if peerBlocks != 4 {
		t.Fatalf("Expect peer to have 3 blocks, got %d", peerBlocks)
	}

	log.Printf("Block A: %x, Block B: %x, Block C: %x", blockA.Hash, blockB.Hash, blockC.Hash)
	go func() {
		<-time.After(1 * time.Second)
		manager.downloader.floodedBlock <- NewFloodBlockReq{FloodBlockReq{FromMinerId: minerC, Block: *blockA, FromMinerAddress: addressC}, make(chan struct{})}
		<-time.After(1 * time.Second)
		manager.downloader.floodedBlock <- NewFloodBlockReq{FloodBlockReq{FromMinerId: minerC, Block: *blockB, FromMinerAddress: addressC}, make(chan struct{})}
		manager.receiveFloodedBlock <- NewFloodBlockReq{FloodBlockReq{FromMinerId: minerC, Block: *blockC, FromMinerAddress: addressC}, make(chan struct{})}
		// Flood a block out of order
		manager.receiveFloodedBlock <- NewFloodBlockReq{FloodBlockReq{FromMinerId: minerC, Block: *blockA, FromMinerAddress: addressC}, make(chan struct{})}
	}()

	go func() {
		for req := range manager.floodBlock {
			log.Printf("Test draining block %x", req.Block.Hash)
			hash := string(req.Block.Hash)
			if hash != string(blockA.Hash) && hash != string(blockB.Hash) && hash != string(blockC.Hash) {
				t.Fatalf("Expected to flood block with hash %x or %x, but flooding %x",
					blockA.Hash, blockB.Hash, req.Block.Hash)
				return
			}

			if req.FromMinerId != minerID {
				t.Fatalf("Expected to flood from miner %s, but got %s", minerID, req.FromMinerId)
				return
			}

			if len(req.ignorePeers) != 1 {
				t.Fatalf("Expected to ignore 1 peer, got %d", len(req.ignorePeers))
				return
			}
		}
	}()

	//done := make(chan struct{})
	//<- done
	<-time.After(5 * time.Second)
	numBlocks := bc1.GetSize()
	if numBlocks != 4 {
		t.Errorf("Expected %d blocks in blockchain, got %d", 4, numBlocks)
	}

	close(done)
}
