package bclib

import (
	"errors"
	"github.com/jlingohr/miner/settings"
	"sync"
)

type Node struct {
	block    *Block
	parent   *Node
	children []*Node
}

type Blockchain struct {
	genesis  *Node
	lastNode *Node
	m        map[string]*Node
	config   settings.Settings
	mux      sync.RWMutex
}

func NewBlockchain(config settings.Settings) *Blockchain {
	node := newNode(NewGenesisBlock(config.GenesisBlockHash), nil)
	bc := &Blockchain{
		genesis:  node,
		lastNode: node,
		m:        map[string]*Node{string(node.block.Hash[:]): node},
		config:   config,
	}
	return bc
}

func newNode(block *Block, parent *Node) *Node {
	node := &Node{
		block:    block,
		parent:   parent,
		children: []*Node{},
	}

	return node
}

func (bc *Blockchain) SetConfig(config settings.Settings) {
	bc.config = config
}

// TODO: return an error??
func (bc *Blockchain) AddBlock(block *Block) error {
	_, ok := bc.m[string(block.Hash[:])]

	if ok {
		return errors.New("block already exists")
	}

	err := bc.validateBlock(block)

	if err != nil {
		return err
	}

	prevNode, ok := bc.m[string(block.PreviousHash[:])]

	if !ok {
		return errors.New("missing previous block")
	}

	node := newNode(block, prevNode)
	prevNode.children = append(prevNode.children, node)

	bc.m[string(node.block.Hash[:])] = node

	return nil
}

func (bc *Blockchain) validateBlock(block *Block) error {
	difficulty := bc.config.PowPerNoOpBlock

	if len(block.Operations) > 0 {
		difficulty = bc.config.PowPerOpBlock
	}

	pow := NewProofOfWork(block, int(difficulty))

	if !pow.Validate() {
		return errors.New("pow failed")
	}

	_, ok := bc.ValidateOperations(block.Operations, block.PreviousHash)

	if !ok {
		return errors.New("operation validation failed")
	}

	return nil
}

//TODO look at uses.
func (bc *Blockchain) GetLastBlocks() []*Block {
	blocks := make([]*Block, 0)

	lastNodes := bc.getLastNodes()

	for _, node := range lastNodes {
		blocks = append(blocks, node.block)
	}

	return blocks
}

func (bc *Blockchain) GetLastHash() []byte {
	//TODO
	lastNodes := bc.getLastNodes()

	return lastNodes[0].block.Hash
}

func (bc *Blockchain) getLastNodes() []*Node {
	dStack := make([]int, 0)
	nStack := make([]*Node, 0)

	bc.mux.RLock()
	lastNode := bc.genesis
	bc.mux.RUnlock()

	lastNodes := []*Node{lastNode}
	maxDepth := 0

	dStack = append(dStack, 0)
	nStack = append(nStack, lastNode)

	for len(nStack) > 0 && len(dStack) > 0 {
		depth := dStack[len(dStack)-1]
		node := nStack[len(nStack)-1]

		dStack = dStack[:len(dStack)-1]
		nStack = nStack[:len(nStack)-1]

		if depth > maxDepth {
			maxDepth = depth
			lastNode = node
			lastNodes = []*Node{node}
		} else if depth == maxDepth {
			lastNodes = append(lastNodes, node)
		}

		for _, child := range node.children {
			nStack = append(nStack, child)
			dStack = append(dStack, depth+1)
		}
	}

	bc.mux.Lock()
	bc.lastNode = lastNode
	bc.mux.Unlock()

	return lastNodes
}

func (bc *Blockchain) GetBlocks() []Block {
	blocks := make([]Block, 0)

	for _, node := range bc.m {
		blocks = append(blocks, *node.block)
	}

	return blocks
}

func (bc *Blockchain) Marshall() ([]byte, error) {
	return MarshallBlocks(bc.GetBlocks())
}

func UnmarshallBlockchain(blocks []Block) (*Blockchain, error) {
	nodeMap := make(map[string]*Node)
	var genesisNode *Node

	for i := range blocks {
		nodeMap[string(blocks[i].Hash[:])] = newNode(&blocks[i], nil)
	}

	for key := range nodeMap {
		node := nodeMap[key]
		if len(node.block.PreviousHash) == 0 {
			genesisNode = node
			continue
		}

		parentNode, ok := nodeMap[string(node.block.PreviousHash[:])]

		if !ok {
			return nil, errors.New("no parent for a node")
		}

		node.parent = parentNode
		parentNode.children = append(parentNode.children, node)
	}

	if genesisNode == nil {
		return nil, errors.New("no genesis block found")
	}

	bc := &Blockchain{
		genesis:  genesisNode,
		lastNode: genesisNode,
		m:        nodeMap,
	}

	bc.getLastNodes()

	return bc, nil
}

func (bc *Blockchain) getLastConfirmedNode(numConfirmedBlocks int) (*Node, bool) {
	opConfirmThresh := numConfirmedBlocks

	lastNodes := bc.getLastNodes()

	if len(lastNodes) == 0 {
		return nil, false
	}

	node := lastNodes[0]

	// backtrack to include confirmed operations only
	for opConfirmThresh > 0 {
		if node.parent == nil {
			return nil, false
		}

		opConfirmThresh -= 1
		node = node.parent
	}

	return node, true
}

func (bc *Blockchain) GetSize() int {
	return len(bc.m)
}

func (bc *Blockchain) HasBlock(hash string) bool {
	_, ok := bc.m[hash]

	return ok
}

func (bc *Blockchain) GetBlock(hash string) (Block, bool) {
	node, ok := bc.m[hash]

	if ok {
		return *node.block, ok
	}

	return Block{}, ok
}

func (bc *Blockchain) GetPreviousBlocks(hash []byte, numBlocks int) []Block {
	node, ok := bc.m[string(hash[:])]

	if !ok {
		return make([]Block, 0)
	}

	result := make([]Block, 0, numBlocks)
	result = append(result, *node.block)

	i := 1

	for ; node != nil && i <= numBlocks; node = node.parent {
		result = append(result, *node.block)
		i++
	}

	return result
}

func (bc *Blockchain) MergeBlocks(blocks []Block) (*Blockchain, error) {
	nodeMap := make(map[string]*Node)
	var genesisNode *Node

	for i := range blocks {
		nodeMap[string(blocks[i].Hash[:])] = newNode(&blocks[i], nil)
	}

	for k, node := range bc.m {
		nodeMap[k] = node
	}

	for key := range nodeMap {
		node := nodeMap[key]
		if len(node.block.PreviousHash) == 0 {
			genesisNode = node
			continue
		}

		parentNode, ok := nodeMap[string(node.block.PreviousHash[:])]

		if !ok {
			return nil, errors.New("no parent for a node")
		}

		node.parent = parentNode
		parentNode.children = append(parentNode.children, node)
	}

	if genesisNode == nil {
		return nil, errors.New("no genesis block found")
	}

	newBc := &Blockchain{
		genesis:  genesisNode,
		lastNode: genesisNode,
		m:        nodeMap,
		config:   bc.config,
	}

	newBc.getLastNodes()

	return newBc, nil
}
