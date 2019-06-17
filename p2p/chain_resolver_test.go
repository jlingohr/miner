package p2p

//import (
//	"log"
//	"testing"
//)
//
//func TestResolveFloodedBlockWithPendingDownload(t *testing.T) {
//	downloadedBlock := make(chan []byte)
//	resolver := NewChainResolver()
//
//	resolver.addPendingDownload(*blockB)
//	resolver.ResolveFloodedBlock(*blockB, downloadedBlock)
//
//	pendingBlocks := resolver.getPendingBlocksForDownload(string(blockA.Hash))
//
//	if len(pendingBlocks) != 1 {
//		t.Errorf("Expected 1 pending block, got %d", len(pendingBlocks))
//	}
//
//	if !contains(pendingBlocks, string(blockB.Hash)) {
//		t.Errorf("Expected pending block with has %x, but none exists", blockB.Hash)
//	}
//}
//
////TODO Need to check that we are properly traversing all permutations of flooded blocks
//
//func TestResolveWhenNotDownloadingPrevBlock(t *testing.T) {
//	downloadedBlock := make(chan []byte)
//	resolver := NewChainResolver()
//
//	go func() {
//		for hash := range downloadedBlock {
//			log.Printf("Request download for hash %x", hash)
//		}
//	}()
//	resolver.addPendingDownload(*blockB)
//	resolver.addWaitingBlockForHash(string(blockB.Hash), string(blockB.PreviousHash))
//	resolver.ResolveFloodedBlock(*blockC, downloadedBlock)
//
//	pendingBlocks := resolver.getPendingBlocksForDownload(string(blockA.Hash))
//	if len(pendingBlocks) != 2 {
//		t.Errorf("Expected 2 pending block, got %d", len(pendingBlocks))
//	}
//
//	if !contains(pendingBlocks, string(blockB.Hash)) {
//		t.Errorf("Expected pending block with has %x, but none exists", blockB.Hash)
//	}
//
//	if !contains(pendingBlocks, string(blockC.Hash)) {
//		t.Errorf("Expected pending block with has %x, but none exists", blockC.Hash)
//	}
//
//	pendingBlocks = resolver.getPendingBlocksForDownload(string(blockB.Hash))
//
//	if len(pendingBlocks) != 1 {
//		t.Errorf("Expected 1 pending block, got %d", len(pendingBlocks))
//	}
//
//	if !contains(pendingBlocks, string(blockC.Hash)) {
//		t.Errorf("Expected pending block with has %x, but none exists", blockC.Hash)
//	}
//}
