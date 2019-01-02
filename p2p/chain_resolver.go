package p2p

import (
	"github.com/jlingohr/miner/bclib"
)

type ChainResolver struct {
	// map of which blocks are being download to a list of blocks that depend on them (like a inverted index). So for each
	// key in the map, once this block is added we can add blocks in the resulting list
	blocksToResolve map[string][]bclib.Block
	// map of which blocks are waiting for a previous block to be downloaded to that block (transitive map into blocksToResolve)
	// So if K -> V is in the map, then V can be added once K is added
	waitingBlocks map[string]string
}

func NewChainResolver() *ChainResolver {
	r := &ChainResolver{
		blocksToResolve: make(map[string][]bclib.Block),
		waitingBlocks:   make(map[string]string),
	}

	return r
}

func (c *ChainResolver) ResolveFloodedBlock(block bclib.Block, downloadBlock chan []byte) {
	// If the blockchain does not have the block and we have requested downloading the previous block
	// we need to associate this block with the requested block
	hash := string(block.Hash)
	prevHash := string(block.PreviousHash)

	if c.hasPendingDownloadForHash(prevHash) {
		c.addPendingDownload(block)
		c.addWaitingBlockForHash(hash, prevHash)
		return
	}

	pendingDownload := c.findPendingDownloadForBlock(block)

	if pendingDownload == "" {
		downloadBlock <- block.PreviousHash
		c.addPendingDownload(block)
		// Add block as waiting block ?
		c.addWaitingBlockForHash(hash, prevHash)
		return
	}

	if c.hasPendingDownloadForHash(pendingDownload) {
		c.addPendingDownload(block)
		c.addWaitingBlockForHash(hash, prevHash)
		return
	}

	// If the blockchain does not have the block and we have not enqueued adding the previous block
	// then we need to download the previous block from known peers
	//log.Printf("Previous hash - %x, hash - %x", block.PreviousHash, block.Hash)
	downloadBlock <- block.PreviousHash
	c.addPendingDownload(block)
	//log.Println()
	//todo also need to add the block once the previous block is done downloading

	//todo need to handle when receive out of order
}

// Resolve any pending blocks we can add given that we recently added a block with this hash
// Since each block has at most one parent, we determine which is the correct block to
// add next by choosing the one that exists only in the longest resolvable chain
func (c *ChainResolver) ResolveAddedBlock(hash string, addBlock chan<- bclib.Block) {
	pendingDownBlocks := c.blocksToResolve[hash]
	// For every block that was waiting for this block to be downloaded
	// we can now also add to the blockchain
	for _, waitingBlock := range pendingDownBlocks {
		delete(c.waitingBlocks, string(waitingBlock.Hash))
		addBlock <- waitingBlock
	}

	delete(c.blocksToResolve, hash)
	// todo add to blockchain, flood if add was successful
}

func (c *ChainResolver) addPendingDownload(block bclib.Block) {
	pendingBlocks, ok := c.blocksToResolve[string(block.PreviousHash)]
	if !ok {
		pendingBlocks = make([]bclib.Block, 0)
		pendingBlocks = append(pendingBlocks, block)
	} else {
		if !contains(pendingBlocks, string(block.Hash)) {
			pendingBlocks = append(pendingBlocks, block)
		}
	}
	c.blocksToResolve[string(block.PreviousHash)] = pendingBlocks
}

func (c *ChainResolver) hasPendingDownloadForHash(hash string) bool {
	_, ok := c.blocksToResolve[hash]
	return ok
}

// If the blockchain does not have the block and we have not requested downloading the previous block
// because it depends on a block we are downloading, then use transitivity to associate
// this block with the downloading block (i.e. waitingBlocks[i] gives the give to the downloading
// block that will resolve the addition to the blockchain
func (c *ChainResolver) findPendingDownloadForBlock(block bclib.Block) string {
	prevHash := string(block.PreviousHash)
	hash := string(block.Hash)

	pendingDownload, _ := c.waitingBlocks[prevHash]
	for pendingDownload != "" {
		blocks, ok := c.blocksToResolve[pendingDownload]
		if !ok {
			blocks = make([]bclib.Block, 0)
		}
		if !contains(blocks, hash) {
			blocks = append(blocks, block)
			c.blocksToResolve[pendingDownload] = blocks
		}
		pendingDownload, _ = c.waitingBlocks[pendingDownload]
	}
	return c.waitingBlocks[hash]
}

func (c *ChainResolver) addWaitingBlockForHash(hash string, previousHash string) {
	c.waitingBlocks[hash] = previousHash
}

func (c *ChainResolver) getPendingBlocksForDownload(hash string) []bclib.Block {
	blocks, ok := c.blocksToResolve[hash]
	if !ok {
		blocks = make([]bclib.Block, 0)
	}
	return blocks
}

func contains(blocks []bclib.Block, hash string) bool {
	for _, b := range blocks {
		if string(b.Hash) == hash {
			return true
		}
	}
	return false
}
