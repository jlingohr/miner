# miner

A simple distributed file system that runs on top of a custom blockchain network. The records file system (RFS) will have a global name space and no access control mechanism. Files in RFS are composed of fixed-length records that can either be read or appended (previously appended records cannot be modified). The blockchain network will be a randomly connected peer-to-peer network that is loosely based on BitCoin: clients submit operations on files, these operations are flooded among nodes, and nodes mine blocks to add operations to the blockchain. Use proof of work to counter sybil nodes/spam and incentivize mining with record coins, which are units of storage in RFS and are necessary for clients to create files/add new records to files.

`go run miner.go [settings]`

Settings is a json file that has the following fields. There are two types of fields: those that do not differ between miners in an instance of an RFS system, and those that do differ between miners.

Settings that do not differ between miners:

* uint8 MinedCoinsPerOpBlock : The number of record coins mined for an op block
* uint8 MinedCoinsPerNoOpBlock : The number of record coins mined for a no-op block
* uint8 NumCoinsPerFileCreate : The number of record coins charged for creating a file
* uint8 GenOpBlockTimeout : Time in milliseconds, the minimum time between op block mining (see diagram above).
* string GenesisBlockHash : The genesis (first) block MD5 hash for this blockchain
* uint8 PowPerOpBlock : The op block difficulty (proof of work setting: number of zeroes)
* uint8 PowPerNoOpBlock : The no-op block difficulty (proof of work setting: number of zeroes)
* uint8 ConfirmsPerFileCreate : The number of confirmations for a create file operation (the number of blocks that must follow the block containing a create file operation along longest chain before the CreateFile call can return successfully)
* uint8 ConfirmsPerFileAppend : The number of confirmations for an append operation (the number of blocks that must follow the block containing an append operation along longest chain before the AppendRec call can return successfully). Note that this append confirm number will always be set to be larger than the create confirm number (above).

Settings that differ between miners:

* string MinerID : The ID of this miner (max 16 characters).
* string[] PeerMinersAddrs : An array of remote IP:port addresses, one per peer miner that this miner should connect to (using the OutgoingMinersIP below)
*string IncomingMinersAddr : The local IP:port where the miner should expect other miners to connect to it (address it should listen on for connections from miners)
* string OutgoingMinersIP : The local IP that the miner should use to connect to peer miners
* string IncomingClientsAddr : The local IP:port where this miner should expect to receive connections from RFS clients (address it should listen on for connections from clients)
