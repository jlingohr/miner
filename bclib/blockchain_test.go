package bclib

//import (
//	"testing"
//)
//
//func TestAddBlock(t *testing.T) {
//	bc := NewBlockchain(createConfig())
//	genesisBlock := bc.genesis.block
//
//	block1 := NewBlock([]Operation{}, genesisBlock.Hash, "1", 1, nil)
//
//	ok := bc.AddBlock(block1)
//
//	if !ok {
//		t.Error("Should be able to add a no-op block to the blockchain.")
//	}
//
//	if len(bc.genesis.children) != 1 {
//		t.Fatal("The first block should be added after the genesis block.")
//	}
//
//	if bc.lastNode == bc.genesis.children[0] {
//		t.Error("AddBlock should not set lastNode = added block.")
//	}
//
//	block2 := NewBlock([]Operation{}, genesisBlock.Hash, "2", 1, nil)
//
//	ok = bc.AddBlock(block2)
//
//	if !ok {
//		t.Error("Should be able to add a valid no-op block to anywhere in the blockchain.")
//	}
//
//	if len(bc.genesis.children) != 2 {
//		t.Fatal("Genesis block should contain two children blocks")
//	}
//
//	block3 := NewBlock([]Operation{}, block2.Hash, "2", 1, nil)
//
//	ok = bc.AddBlock(block3)
//
//	if !ok {
//		t.Fatal("Should be able to add a valid no-op block to anywhere in the blockchain.")
//	}
//
//	// block2 should have one child (block3)
//	node2 := bc.genesis.children[1]
//	if len(node2.children) != 1 || node2.children[0].block != block3 {
//		t.Fatal("AddBlock should add blocks to the correct nodes.")
//	}
//}
//
//func TestValidate(t *testing.T) {
//	bc := buildTestBlockchain()
//
//	noOpBlock := NewBlock([]Operation{}, bc.lastNode.block.Hash, "1", 1, nil)
//
//	ok := bc.validateBlock(noOpBlock)
//
//	if !ok {
//		t.Fatal("validateBlock should return true for a valid no-opBlock")
//	}
//
//	noOpBlockNode := newNode(noOpBlock, bc.lastNode)
//
//	// add noOpBlockNode to the blockchain
//	bc.lastNode.children = append(bc.lastNode.children, noOpBlockNode)
//	bc.m[string(noOpBlock.Hash[:])] = noOpBlockNode
//	bc.lastNode = noOpBlockNode
//
//	// miner with minerID = "1" should now have 1 coin
//	op1 := NewOperation(Create, "1", "f5", []byte("f5"))
//	block1 := NewBlock([]Operation{op1}, noOpBlock.Hash, "1", 1, nil)
//
//	ok = bc.validateBlock(block1)
//
//	if !ok {
//		t.Fatal("validateBlock should return true for a valid block.")
//	}
//
//	op2 := NewOperation(Create, "1", "f6", []byte("f6"))
//	block2 := NewBlock([]Operation{op1, op2}, noOpBlock.Hash, "1", 1, nil)
//
//	// block2 is not valid since miner with minerID = "1" has only one coin
//	ok = bc.validateBlock(block2)
//
//	if ok {
//		t.Fatal("validateBlock should return false for an invalid block.")
//	}
//
//	// a block containing existing create operation should fail
//	op3 := NewOperation(Create, "1", "f1", []byte("somedata"))
//	block3 := NewBlock([]Operation{op3}, noOpBlock.Hash, "1", 1, nil)
//
//	ok = bc.validateBlock(block3)
//
//	if ok {
//		t.Fatal("validateBlock should return false for a block containing existing create operation.")
//	}
//}
//
//func TestGetLastNode(t *testing.T) {
//	bc := buildTestBlockchain()
//
//	// test on a blockchain without forks
//	lastNode := bc.lastNode
//	bc.lastNode = bc.genesis
//
//	testLastNode := bc.getLastNodes()
//	bc.lastNode = bc.genesis
//
//	if len(testLastNode) != 1 && testLastNode[0] != lastNode {
//		t.Fatal("Should be able to correctly retrieve the last node along the longest chain.")
//	}
//
//	// add a fork in the middle
//	secondNode := bc.genesis.children[0]
//	block1 := NewBlock([]Operation{}, secondNode.block.Hash, "10", 1, nil)
//	node1 := newNode(block1, secondNode)
//
//	secondNode.children = append(secondNode.children, node1)
//	bc.m[string(block1.Hash[:])] = node1
//
//	testLastNode = bc.getLastNodes()
//	bc.lastNode = bc.genesis
//
//	if len(testLastNode) != 1 || testLastNode[0] != lastNode {
//		t.Fatal("Should be able to correctly retrieve the last node with a fork in the middle.")
//	}
//
//	block2 := NewBlock([]Operation{}, block1.Hash, "11", 1, nil)
//	node2 := newNode(block2, node1)
//
//	// add a node to node1, now we have two longest chains
//	node1.children = append(node1.children, node2)
//	bc.m[string(block2.Hash[:])] = node2
//
//	testLastNode = bc.getLastNodes()
//	bc.lastNode = bc.genesis
//
//	if len(testLastNode) != 2 || testLastNode[0] != node2 || testLastNode[1] != lastNode {
//		t.Fatal("Should be able to correctly retrieve the last node when there are multiple longest chains.")
//	}
//
//	secondNode.children[1] = secondNode.children[0]
//	secondNode.children[0] = node1
//
//	testLastNode = bc.getLastNodes()
//
//	if len(testLastNode) != 2 || testLastNode[0] != lastNode || testLastNode[1] != node2 {
//		t.Fatal("Should be able to correctly retrieve the last node when there are multiple longest chains.")
//	}
//}
//
//func TestMarshallUnmarshall(t *testing.T) {
//	bc := buildTestBlockchain()
//
//	bcBytes, err := bc.Marshall()
//
//	if err != nil {
//		t.Fatal("Should be able to marshall the blockchain data structure.")
//	}
//
//	bcCopy, err := UnmarshallBlockchain(bcBytes)
//
//	if err != nil {
//		t.Fatal("Should be able to unmarshall the blockchain data structure.")
//	}
//
//	if bcCopy.genesis.parent != nil {
//		t.Fatal("Genesis node should not have a parent.")
//	}
//
//	if len(bcCopy.genesis.children) != 1 {
//		t.Fatal("Genesis block should have exactly one child.")
//	}
//
//	if string(bcCopy.lastNode.block.Hash[:]) != string(bc.lastNode.block.Hash[:]) {
//		t.Fatal("Last node should be set correctly, and the hash should be equal to the original block")
//	}
//
//}
//
//func buildTestBlockchain() *Blockchain {
//	bc := NewBlockchain(createConfig())
//	genesisBlock := bc.genesis.block
//
//	op1 := NewOperation(Create, "1", "f1", []byte("f1"))
//	op2 := NewOperation(Create, "2", "f2", []byte("f2"))
//
//	block1 := NewBlock([]Operation{}, genesisBlock.Hash, "1", 1, nil)
//	block2 := NewBlock([]Operation{op1}, block1.Hash, "2", 1, nil)
//	block3 := NewBlock([]Operation{op2}, block2.Hash, "3", 1, nil)
//
//	node1 := newNode(block1, bc.genesis)
//	node2 := newNode(block2, node1)
//	node3 := newNode(block3, node2)
//
//	bc.genesis.children = append(bc.genesis.children, node1)
//	node1.children = append(node1.children, node2)
//	node2.children = append(node2.children, node3)
//
//	bc.m[string(block1.Hash[:])] = node1
//	bc.m[string(block2.Hash[:])] = node2
//	bc.m[string(block3.Hash[:])] = node3
//
//	bc.lastNode = node3
//
//	return bc
//}

//func createConfig() settings.Settings {
//	config := settings.Settings{
//		NumCoinsPerFileCreate:  1,
//		MinedCoinsPerNoOpBlock: 1,
//		MinedCoinsPerOpBlock:   1,
//		PowPerOpBlock:          1,
//		PowPerNoOpBlock:        1,
//		MinerID:                "genesis",
//		GenesisBlockHash:       "83218ac34c1834c26781fe4bde918ee4",
//	}
//
//	return config
//}
