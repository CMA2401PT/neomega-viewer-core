package topview

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"slices"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

// Table 是 Java 状态哈希到不透明 sRGB 的表，颜色放在低 24 位 0xRRGGBB。
// 颜色在建表时算好；是否可见由渲染结果的 Alpha 表示，RGB 为 0 仍是有效的黑色。
type Table map[uint32]uint32

type header struct {
	Magic   [4]byte
	Version uint32
	Count   uint32
}

type entry struct {
	Hash  uint32
	Color uint32
}

func (t Table) Encode(w io.Writer) error {
	// 固定记录顺序，使相同颜色表重新生成后的字节保持一致。
	buffer := bufio.NewWriter(w)
	if err := binary.Write(buffer, binary.LittleEndian, header{
		Magic:   [4]byte{'N', 'V', 'C', 'L'},
		Version: 1,
		Count:   uint32(len(t)),
	}); err != nil {
		return err
	}
	for _, hash := range slices.Sorted(maps.Keys(t)) {
		if err := binary.Write(buffer, binary.LittleEndian, entry{Hash: hash, Color: t[hash]}); err != nil {
			return err
		}
	}
	return buffer.Flush()
}

func Decode(r io.Reader) (Table, error) {
	// 只识别文件格式，状态是否可用由调用方查色时决定。
	buffer := bufio.NewReader(r)
	var head header
	if err := binary.Read(buffer, binary.LittleEndian, &head); err != nil {
		return nil, err
	}
	if string(head.Magic[:]) != "NVCL" {
		return nil, fmt.Errorf("invalid NVCL magic")
	}
	if head.Version != 1 {
		return nil, fmt.Errorf("unsupported NVCL version %d", head.Version)
	}
	table := make(Table, head.Count)
	for range head.Count {
		var item entry
		if err := binary.Read(buffer, binary.LittleEndian, &item); err != nil {
			return nil, err
		}
		table[item.Hash] = item.Color
	}
	return table, nil
}

func (t Table) Color(block ir.IRBlock) (uint32, bool) {
	// Color 把含水和旧版本状态折到建表时的目标状态再查表。
	// 没有颜色时返回 false，调用方自己决定回退色。空气不在表里。
	target, ok := java.NormalizeSingleLayer(block)
	if !ok {
		return 0, false
	}
	color, ok := t[target.Hash()]
	return color, ok
}
