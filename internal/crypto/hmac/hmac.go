package hmac

import (
	"crypto/subtle"
	"hash"

	"project/internal/security"
)

// from GOROOT/src/crypto/hmac/hmac.go
//
// use security.Bytes to replace []byte for store key more secure.
//
// FIPS 198-1:
// https://csrc.nist.gov/publications/fips/fips198-1/FIPS-198-1_final.pdf

// key is zero padded to the block size of the hash function
// iPad = 0x36 byte repeated for key length
// oPad = 0x5c byte repeated for key length
// hmac = H([key ^ oPad] H([key ^ iPad] text))

// MarshalAble is the combination of encoding.BinaryMarshaler and
// encoding.BinaryUnmarshaler. Their method definitions are repeated here to
// avoid a dependency on the encoding package.
type marshalAble interface {
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

type hmac struct {
	oPad  *security.Bytes
	iPad  *security.Bytes
	outer hash.Hash
	inner hash.Hash

	// If marshaled is true, then oPad and iPad do not contain a padded
	// copy of the key, but rather the marshaled state of outer/inner after
	// oPad/iPad has been fed into it.
	marshaled bool
}

func (h *hmac) Sum(in []byte) []byte {
	origLen := len(in)
	in = h.inner.Sum(in)

	oPad := h.oPad.Get()
	defer h.oPad.Put(oPad)

	if h.marshaled {
		if err := h.outer.(marshalAble).UnmarshalBinary(oPad); err != nil {
			panic(err)
		}
	} else {
		h.outer.Reset()
		h.outer.Write(oPad)
	}
	h.outer.Write(in[origLen:])
	return h.outer.Sum(in[:origLen])
}

func (h *hmac) Write(p []byte) (n int, err error) {
	return h.inner.Write(p)
}

func (h *hmac) Size() int      { return h.outer.Size() }
func (h *hmac) BlockSize() int { return h.inner.BlockSize() }

func (h *hmac) Reset() {
	iPad := h.iPad.Get()
	defer h.iPad.Put(iPad)

	if h.marshaled {
		if err := h.inner.(marshalAble).UnmarshalBinary(iPad); err != nil {
			panic(err)
		}
		return
	}

	h.inner.Reset()
	h.inner.Write(iPad)

	// If the underlying hash is marshalAble, we can save some time by
	// saving a copy of the hash state now, and restoring it on future
	// calls to Reset and Sum instead of writing iPad/oPad every time.
	//
	// If either hash is unmarshalAble for whatever reason,
	// it's safe to bail out here.
	marshalAbleInner, innerOK := h.inner.(marshalAble)
	if !innerOK {
		return
	}
	marshalAbleOuter, outerOK := h.outer.(marshalAble)
	if !outerOK {
		return
	}

	iMarshal, err := marshalAbleInner.MarshalBinary()
	if err != nil {
		return
	}
	defer security.CoverBytes(iMarshal)

	h.outer.Reset()

	oPad := h.oPad.Get()
	defer h.oPad.Put(oPad)

	h.outer.Write(oPad)

	oMarshal, err := marshalAbleOuter.MarshalBinary()
	if err != nil {
		return
	}
	defer security.CoverBytes(oMarshal)

	// Marshaling succeeded; save the marshaled state for later
	h.iPad = security.NewBytes(iMarshal)
	h.oPad = security.NewBytes(oMarshal)
	h.marshaled = true
}

// New returns a new HMAC hash using the given hash.Hash type and key.
// Note that unlike other hash implementations in the standard library,
// the returned Hash does not implement encoding.BinaryMarshaler
// or encoding.BinaryUnmarshaler.
func New(h func() hash.Hash, key []byte) hash.Hash {
	hm := new(hmac)
	hm.outer = h()
	hm.inner = h()
	blockSize := hm.inner.BlockSize()
	if len(key) > blockSize {
		// If key is too large, hash it.
		hm.outer.Write(key)
		key = hm.outer.Sum(nil)
	}
	iPad := make([]byte, blockSize)
	oPad := make([]byte, blockSize)
	defer func() {
		security.CoverBytes(iPad)
		security.CoverBytes(oPad)
	}()
	copy(iPad, key)
	copy(oPad, key)
	for i := range iPad {
		iPad[i] ^= 0x36
	}
	for i := range oPad {
		oPad[i] ^= 0x5c
	}
	hm.inner.Write(iPad)
	// save
	hm.iPad = security.NewBytes(iPad)
	hm.oPad = security.NewBytes(oPad)
	// cover at once
	security.CoverBytes(iPad)
	security.CoverBytes(oPad)
	return hm
}

// Equal compares two MACs for equality without leaking timing information.
func Equal(mac1, mac2 []byte) bool {
	// We don't have to be constant time if the lengths of the MACs are
	// different as that suggests that a completely different hash function
	// was used.
	return subtle.ConstantTimeCompare(mac1, mac2) == 1
}
