package geometry

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

type custom64 uint64

func TestIndependentBinaryLayout(t *testing.T) {
	// 手写格式样本不经过编码器，独立约束头部和 64 位键的布局。
	fixture := []byte{
		'N', 'V', 'G', 'T', 2, 0, 8, 0,
		1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		8, 7, 6, 5, 4, 3, 2, 1, 0, 0, 0, 0,
	}
	tables, err := DecodeTables[custom64](bytes.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if ids, err := tables.ModelIDs(0x0102030405060708); err != nil || len(ids) != 0 {
		t.Fatalf("64-bit key: %v", err)
	}
	var encoded bytes.Buffer
	if err := tables.Encode(&encoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded.Bytes(), fixture) {
		t.Fatal("layout differs from fixture")
	}
	if _, err := DecodeTables[uint32](bytes.NewReader(fixture)); err == nil {
		t.Fatal("width mismatch accepted")
	}
	for _, offset := range []int{0, 4, 6} {
		bad := bytes.Clone(fixture)
		bad[offset] = 99
		if _, err := DecodeTables[uint64](bytes.NewReader(bad)); err == nil {
			t.Fatalf("header field %d accepted", offset)
		}
	}
	for _, length := range []int{0, 31, len(fixture) - 1} {
		if _, err := DecodeTables[uint64](bytes.NewReader(fixture[:length])); err == nil {
			t.Fatalf("truncation at %d accepted", length)
		}
	}
	if binary.Size(tableHeader{}) != 32 || binary.Size(wireQuad{}) != 57 || binary.Size(wireBoundary{}) != 36 {
		t.Fatal("fixed record widths changed")
	}
	if err := tables.Encode(failingWriter{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write error lost: %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
