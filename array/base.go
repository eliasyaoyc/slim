package array

import (
	"encoding/binary"
	"reflect"

	"github.com/openacid/errors"
	"github.com/openacid/low/bitmap"
	"github.com/openacid/slim/encode"
)

// endian is the default endian for array
var endian = binary.LittleEndian

// Base is the base of: Array and U16 etc.
//
// Since 0.2.0
type Base struct {
	Array32
	EltEncoder encode.Encoder
}

const (
	bmShift = uint(6) // log₂64
	bmMask  = int32(63)
)

// bmBit calculates bitamp word index and the bit index in the word.
func bmBit(idx int32) (int32, int32) {
	c := idx >> bmShift
	r := idx & bmMask
	return c, r
}

// InitIndex initializes index bitmap for an array.
// Index must be an ascending int32 slice, otherwise, it return
// the ErrIndexNotAscending error
//
// Since 0.2.0
func (a *Base) InitIndex(index []int32) error {

	for i := 0; i < len(index)-1; i++ {
		if index[i] >= index[i+1] {
			return ErrIndexNotAscending
		}
	}

	a.Bitmaps = bitmap.Of(index)
	a.Offsets = bitmap.IndexRank64(a.Bitmaps)
	a.Cnt = int32(len(index))

	// Be compatible to previous issue:
	// Since v0.2.0, Offsets is not exactly the same as bitmap ranks.
	// It is 0 for empty bitmap word.
	// But bitmap ranks set rank[i*64] to rank[(i-1)*64] for empty word.
	for i, word := range a.Bitmaps {
		if word == 0 {
			a.Offsets[i] = 0
		}
	}

	return nil
}

// Init initializes an array from the "indexes" and "elts".
// The indexes must be an ascending int32 slice,
// otherwise, return the ErrIndexNotAscending error.
// The "elts" is a slice.
//
// Since 0.2.0
func (a *Base) Init(indexes []int32, elts interface{}) error {

	rElts := reflect.ValueOf(elts)
	if rElts.Kind() != reflect.Slice {
		panic("elts is not a slice")
	}

	n := rElts.Len()
	if len(indexes) != n {
		return ErrIndexLen
	}

	err := a.InitIndex(indexes)
	if err != nil {
		return err
	}

	if len(indexes) == 0 {
		return nil
	}

	var encoder encode.Encoder

	if a.EltEncoder == nil {
		var err error
		encoder, err = encode.NewTypeEncoderEndian(rElts.Index(0).Interface(), endian)
		if err != nil {
			// TODO wrap
			return err
		}
	} else {
		encoder = a.EltEncoder
	}

	_, err = a.InitElts(elts, encoder)
	if err != nil {
		return errors.Wrapf(err, "failure Init Array")
	}

	return nil
}

// InitElts initialized a.Elts, by encoding elements in to bytes.
//
// Since 0.2.0
func (a *Base) InitElts(elts interface{}, encoder encode.Encoder) (int, error) {

	rElts := reflect.ValueOf(elts)
	n := rElts.Len()
	eltsize := encoder.GetEncodedSize(nil)
	sz := eltsize * n

	b := make([]byte, 0, sz)
	for i := 0; i < n; i++ {
		ee := rElts.Index(i).Interface()
		bs := encoder.Encode(ee)
		b = append(b, bs...)
	}
	a.Elts = b

	return n, nil
}

// Get retrieves the value at "idx" and return it.
// If this array has a value at "idx" it returns the value and "true",
// otherwise it returns "nil" and "false".
//
// Since 0.2.0
func (a *Base) Get(idx int32) (interface{}, bool) {

	bs, ok := a.GetBytes(idx, a.EltEncoder.GetEncodedSize(nil))
	if ok {
		_, v := a.EltEncoder.Decode(bs)
		return v, true
	}

	return nil, false
}

// GetBytes retrieves the raw data of value in []byte at "idx" and return it.
//
// Performance note
//
// Involves 2 memory access:
//	 a.Bitmaps
//	 a.Elts
//
// Involves 0 alloc
//
// Since 0.2.0
func (a *Base) GetBytes(idx int32, eltsize int) ([]byte, bool) {
	r, b := bitmap.Rank64(a.Bitmaps, a.Offsets, idx)
	if b == 0 {
		return nil, false
	}

	stIdx := int32(eltsize) * r
	return a.Elts[stIdx : stIdx+int32(eltsize)], true
}
