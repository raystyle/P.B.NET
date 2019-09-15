package node

type db struct {
}

type message struct {
	GUID      []byte
	Message   []byte
	Signature []byte
}

func (db *db) SelectNodeMessage(guid []byte, index uint64) (*message, error) {

	return nil, nil
}
