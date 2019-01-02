package bclib

type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

type TXOutput struct {
	Value        int
	ScriptPubKey string
}

type TXInput struct {
	TXid      []byte
	Vout      int
	ScriptSig string
}
