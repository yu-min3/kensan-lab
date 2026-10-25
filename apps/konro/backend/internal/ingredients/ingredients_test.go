package ingredients

import "testing"

func TestQuantities(t *testing.T) {
	cases := []struct{ raw, scale, text, kind string }{{"玉ねぎ １/２個", "2", "玉ねぎ 1個", "exact"}, {"醤油 大さじ1/2", "3", "しょうゆ 大さじ1.5", "exact"}, {"鶏もも肉 200g", "3/2", "鶏もも肉 300g", "exact"}, {"玉ねぎ 1〜2個", "2", "玉ねぎ 2〜4個", "range"}, {"塩 少々", "2", "塩 少々", "vague"}, {"にんじんまたは大根 100g", "2", "にんじんまたは大根 100g", "unparsed"}, {"【調味料】", "2", "【調味料】", "group"}, {"玉ねぎ 1/0個", "2", "玉ねぎ 1/0個", "unparsed"}}
	for _, c := range cases {
		line := Parse(c.raw)
		scaled, e := line.Scaled(c.scale)
		if e != nil || scaled.Kind != c.kind || scaled.Text() != c.text {
			t.Errorf("%s -> %+v %v", c.raw, scaled, e)
		}
	}
	if Add("1/3", "2/3") != "1" {
		t.Fatal("fraction precision lost")
	}
	if _, e := Multiply("1", "0"); e == nil {
		t.Fatal("zero scale accepted")
	}
	if Parse("玉ねぎ 1個").Key() == Parse("玉ねぎ 100g").Key() {
		t.Fatal("incompatible units merged")
	}
	if Parse("鶏もも肉 100g").Key() == Parse("鶏むね肉 100g").Key() {
		t.Fatal("different cuts merged")
	}
}
