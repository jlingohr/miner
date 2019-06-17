package p2p

import (
	"github.com/jlingohr/rfs-miner/bclib"
	"github.com/jlingohr/rfs-miner/settings"
	"log"
	"sync"
	"time"
)

type ChainManager struct {
	blockchain *bclib.Blockchain
	config     settings.Settings

	//downloader    *Downloader
	chainResolver *ChainResolver

	floodBlock            chan MakeFloodBlockReq
	syncBlockchain      chan SynchronizeBlockchain
	addBlock            chan bclib.Block
	receiveFloodedBlock chan bclib.Block
	sendBlock           chan bclib.Block
	continueMining      chan struct{}
	newBlockAdded       chan struct{}
	downloadBlock       chan bclib.Block
	downloadedBlocks    chan []bclib.Block

	isMining bool

	mux  sync.RWMutex
	fmux sync.RWMutex
}

func NewChainManager(bc *bclib.Blockchain, config settings.Settings) *ChainManager {
	//makeRequestChan := make(chan MakeFloodBlockReq)
	manager := &ChainManager{
		blockchain: bc,
		config:     config,
		//floodBlock:            makeRequestChan,
		syncBlockchain:      make(chan SynchronizeBlockchain),
		addBlock:            make(chan bclib.Block, 100),  //This needs to be buffered since the goroutine consuming also writes
		receiveFloodedBlock: make(chan bclib.Block, 1000), // This also needs to be buffered for the same reason
		continueMining:      make(chan struct{}),
		downloadedBlocks:    make(chan []bclib.Block),
		sendBlock:           make(chan bclib.Block),

		//downloader:    NewDownloader(config.MinerID, config.IncomingMinersAddr, makeRequestChan),
		chainResolver: NewChainResolver(),
	}

	//go manager.updatePendingBlocks()
	go manager.manageBlockchain()
	return manager
}

func (m *ChainManager) manageBlockchain() {
	//queuedAdding := make(map[string]struct{}) // Track which blocks are enqueued to add to the blockchain
	// map of which blocks are being download to a list of blocks that depend on them (like a inverted index)
	//blocksToResolve := make(map[string][]bclib.Block)
	// map of which blocks are waiting for a previous block to be downloaded to that block (transitive map into blocksToResolve)
	//waitingBlocks := make(map[string]string)

	ticker := time.NewTicker(10 * time.Second)
	replayQueue := make([]bclib.Block, 0)
	replayMap := make(map[string]struct{})

	for {
		select {
		case <-ticker.C:
			log.Printf("ChainManager heartbeat")
		case newBlocks := <-m.syncBlockchain:
			blocks := newBlocks.PeerBlocks
			bc, err := bclib.UnmarshallBlockchain(blocks)
			if err != nil {
				log.Println(err)
				continue
			}
			bc.SetConfig(m.config)

			m.mux.Lock()
			m.blockchain = bc
			m.mux.Unlock()

			log.Printf("%s has %d blocks after synchronization", m.config.MinerID, bc.GetSize())

			close(newBlocks.done)

		case block := <-m.receiveFloodedBlock:
			m.mux.RLock()
			hasBlock := m.blockchain.HasBlock(string(block.Hash[:]))
			canAdd := !hasBlock && m.blockchain.HasBlock(string(block.PreviousHash[:]))
			m.mux.RUnlock()

			if canAdd {
				// If the blockchain does not have the block and the blockchain has the previous block
				// then we can add the block
				m.addBlock <- block
				//queuedAdding[string(block.Hash)] = struct{}{}
				log.Printf("%s has %d pending add blocks", m.config.MinerID, len(m.addBlock))
			} else if !hasBlock {
				_, ok := replayMap[string(block.PreviousHash)]
				if ok {
					log.Printf("%s adding %x to replay map", m.config.MinerID, block.Hash)
					replayMap[string(block.Hash)] = struct{}{}
					m.receiveFloodedBlock <- block
					continue
				}

				if len(replayQueue) == 0 && !ok {
					log.Printf("%s missing previous block %x", m.config.MinerID, block.PreviousHash)
					m.downloadBlock <- block
				}

				replayQueue = append(replayQueue, block)
				replayMap[string(block.Hash)] = struct{}{}
			}

		case blocks := <-m.downloadedBlocks:
			log.Printf("%s resolving %d blocks from block %x", m.config.MinerID, len(blocks), replayQueue[0])
			replaySlice := make([]bclib.Block, 0, len(replayQueue)+len(blocks))
			for _, block := range blocks {
				replaySlice = append(replaySlice, block)
				replayMap[string(block.Hash)] = struct{}{}
			}

			for _, block := range replayQueue {
				replaySlice = append(replaySlice, block)
			}

			for _, block := range replaySlice {
				m.receiveFloodedBlock <- block
			}

			replayQueue = make([]bclib.Block, 0)

		case block := <-m.addBlock:
			log.Println("InsideADDBLOCK")
			m.mux.Lock()
			err := m.blockchain.AddBlock(&block)
			m.mux.Unlock()

			if err != nil {
				log.Printf("%s failed to add block with hash %x mined by %s - %s", m.config.MinerID, block.Hash, block.MinerID, err)
			} else {
				log.Printf("%s added block with hash %x mined by %s", m.config.MinerID, block.Hash, block.MinerID)
				log.Printf("Current last hash %x", m.blockchain.GetLastHash()[:])

				m.newBlockAdded <- struct{}{}
				m.sendBlock <- block
			}

			log.Printf("%s has %d blocks", m.config.MinerID, m.blockchain.GetSize())

			m.continueMining <- struct{}{}
		}
	}
}

/*
 * ChainManager thread safe RFSLedger implementation
 */

func (m *ChainManager) GetLastHash() []byte {
	m.mux.RLock()
	hash := m.blockchain.GetLastHash()
	m.mux.RUnlock()

	return hash
}

func (m *ChainManager) GetLastBlocks() []*bclib.Block {
	m.mux.RLock()
	blocks := m.blockchain.GetLastBlocks()
	m.mux.RUnlock()

	return blocks
}

func (m *ChainManager) ValidateOperations(filteredOperations []bclib.Operation, prevHash []byte) ([]bclib.Operation, bool) {
	m.mux.RLock()
	ops, ok := m.blockchain.ValidateOperations(filteredOperations, prevHash)
	m.mux.RUnlock()

	return ops, ok
}

func (m *ChainManager) GetFile(fname string) (bclib.FileOps, bool) {
	m.mux.RLock()
	ops, ok := m.blockchain.GetFile(fname)
	m.mux.RUnlock()

	return ops, ok
}

func (m *ChainManager) ListFiles() []string {
	m.mux.RLock()
	fnames := m.blockchain.ListFiles()
	m.mux.RUnlock()

	return fnames
}

func (m *ChainManager) CheckOpConfirmed(op bclib.Operation) (confirmed, exists, otherMiner bool) {
	m.mux.RLock()
	confirmed, exists, otherMiner = m.blockchain.CheckOpConfirmed(op)
	m.mux.RUnlock()

	return
}

func (m *ChainManager) NumRecs(fname string) (int, bool) {
	m.mux.RLock()
	numRecs, exists := m.blockchain.NumRecs(fname)
	m.mux.RUnlock()

	return numRecs, exists
}

func (m *ChainManager) GetPreviousBlocks(hash []byte, numBlocks int) []bclib.Block {
	m.mux.RLock()
	blocks := m.blockchain.GetPreviousBlocks(hash, numBlocks)
	m.mux.RUnlock()

	//Need to reverse the array, otherwise we flood replay map too much
	for i := 0; i < len(blocks)/2; i++ {
		j := len(blocks) - i - 1
		blocks[i], blocks[j] = blocks[j], blocks[i]
	}

	return blocks
}
