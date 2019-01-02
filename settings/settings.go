package settings

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Settings struct {
	MinedCoinsPerOpBlock   uint8
	MinedCoinsPerNoOpBlock uint8
	NumCoinsPerFileCreate  uint8
	GenOpBlockTimeout      uint8
	GenesisBlockHash       string
	PowPerOpBlock          uint8
	PowPerNoOpBlock        uint8
	ConfirmsPerFileCreate  uint8
	ConfirmsPerFileAppend  uint8
	// Unique settings
	MinerID             string
	PeerMinersAddrs     []string
	IncomingMinersAddr  string
	OutgoingMinersIP    string
	IncomingClientsAddr string
}

func LoadSettings(fname string) Settings {
	file, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Fatal(err)
	}
	var settings Settings
	json.Unmarshal(file, &settings)
	return settings
}
