package solidtable

import "testing"

func TestBundledTable(t *testing.T) {
	table, err := Bundled()
	if err != nil {
		t.Fatal(err)
	}
	if len(table) == 0 {
		t.Fatal("bundled solid table is empty")
	}
}
