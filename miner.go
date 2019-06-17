package main

import (
	"github.com/jlingohr/rfs-miner/bclib"
	"github.com/jlingohr/rfs-miner/p2p"
	"github.com/jlingohr/rfs-miner/settings"
	"log"
	"os"
)

const (
	MAX_BLOCK_SIZE = 100
)

// Miner has two roles
// 1) Mine RFS coins for clients to consumer
// 2) Participate in the network to help maintain the blockchain
type Miner struct {
	MinerID string

	Balance    uint8
	operations chan bclib.Operation // Channel of new operations flooded by peers
	blocks     chan bclib.Block     // Channel of new blocks flooded by peers
	done       chan struct{}
}

func main() {
	args := os.Args[1:]
	if len(args) != 1 {
		log.Fatal("Expect: go run miner.go [settings].")
	}

	config := settings.LoadSettings(args[0])
	//logger := logging.SetupLogging("Node", config.MinerID, true, govec.DEBUG, true)
	//opts := govec.GetDefaultLogOptions()
	//p2p.SetLogger(logger, opts)

	_, err := p2p.NewMinerServer(config)
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan struct{})
	<-done
}
