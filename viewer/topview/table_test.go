package topview

import (
	"bytes"
	"maps"
	"testing"

	"neomega-blocks/ir"
)

func TestWireFormat(t *testing.T) {
	// 独立字节样本同时固定小端布局、记录顺序和颜色的低 24 位表示。
	wire := []byte{
		'N', 'V', 'C', 'L', 1, 0, 0, 0, 2, 0, 0, 0,
		4, 3, 2, 1, 0x56, 0x34, 0x12, 0,
		8, 7, 6, 5, 0xef, 0xcd, 0xab, 0,
	}
	want := Table{0x05060708: 0xabcdef, 0x01020304: 0x123456}
	var first, second bytes.Buffer
	if err := want.Encode(&first); err != nil {
		t.Fatal(err)
	}
	if err := want.Encode(&second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), wire) || !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatalf("unstable or incorrect encoding: %x; %x", first.Bytes(), second.Bytes())
	}
	got, err := Decode(bytes.NewReader(first.Bytes()))
	if err != nil || !maps.Equal(got, want) {
		t.Fatalf("decoded %v, error %v; want %v", got, err, want)
	}
	for _, offset := range []int{0, 4} {
		invalid := bytes.Clone(wire)
		invalid[offset]++
		if _, err := Decode(bytes.NewReader(invalid)); err == nil {
			t.Fatalf("accepted invalid header at byte %d", offset)
		}
	}
}

func TestColorNormalizesWaterlogged(t *testing.T) {
	// 表只存目标状态，含水属性不应增加颜色记录。
	stone := ir.IRBlock("minecraft:stone []").Tidy()
	table := Table{stone.Hash(): 0x123456}
	for _, text := range []string{
		"minecraft:stone []",
		`minecraft:stone ["waterlogged"="true"]`,
		`minecraft:stone ["waterlogged"="false"]`,
	} {
		got, ok := table.Color(ir.IRBlock(text).Tidy())
		if !ok || got != 0x123456 {
			t.Fatalf("%s: got %#06x, %v; want 0x123456, true", text, got, ok)
		}
	}
}

func TestBundled(t *testing.T) {
	// 约束可辨认的颜色关系，允许资源更新带来具体像素值变化。
	table, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	colors := make(map[string]uint32)
	for _, text := range []string{
		"minecraft:stone []",
		`minecraft:grass_block ["snowy"="false"]`,
		`minecraft:water ["level"="0"]`,
		`minecraft:oak_log ["axis"="y"]`,
		`minecraft:oak_log ["axis"="x"]`,
		"minecraft:poppy []",
		"minecraft:grass []",
		"minecraft:short_grass []",
	} {
		color, ok := table.Color(ir.IRBlock(text).Tidy())
		if !ok {
			t.Fatalf("missing color: %s", text)
		}
		colors[text] = color
		t.Logf("%s: 0x%06X", text, color)
	}
	stone := colors["minecraft:stone []"]
	r, g, b := stone>>16&255, stone>>8&255, stone&255
	if max(r, g, b)-min(r, g, b) > 24 || stone == 0 || stone == 0xffffff {
		t.Errorf("stone is not a midtone gray: %#06x", stone)
	}
	grass := colors[`minecraft:grass_block ["snowy"="false"]`]
	r, g, b = grass>>16&255, grass>>8&255, grass&255
	if g <= r || g <= b {
		t.Errorf("grass block is not green: %#06x", grass)
	}
	water := colors[`minecraft:water ["level"="0"]`]
	r, g, b = water>>16&255, water>>8&255, water&255
	if b <= r || b <= g {
		t.Errorf("water is not blue: %#06x", water)
	}
	if colors[`minecraft:oak_log ["axis"="y"]`] == colors[`minecraft:oak_log ["axis"="x"]`] {
		t.Error("upright and sideways oak logs have the same color")
	}
	poppy := colors["minecraft:poppy []"]
	r, g, b = poppy>>16&255, poppy>>8&255, poppy&255
	if r <= g || r <= b {
		t.Errorf("poppy is not red: %#06x", poppy)
	}
	if _, ok := table.Color(ir.IRBlock("minecraft:air []").Tidy()); ok {
		t.Error("air has a color")
	}
	if colors["minecraft:grass []"] != colors["minecraft:short_grass []"] {
		t.Error("legacy grass does not use the short grass color")
	}
	t.Logf("bundled colors: %d", len(table))
}
