package node

type message struct {
	GUID      []byte
	Message   []byte
	Signature []byte
}

type db struct {
}

func newDB(ctx *NODE, cfg *Config) (*db, error) {
	return new(db), nil
}

func (db *db) SelectNodeMessage(guid []byte, index uint64) (*message, error) {

	return nil, nil
}
