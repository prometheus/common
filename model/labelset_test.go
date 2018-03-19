package model

import "testing"

func TestRemoveEmptyLabels(t *testing.T) {
	ls := LabelSet{
		LabelName("one"):   LabelValue("two"),
		LabelName("three"): LabelValue("four"),
		LabelName("five"):  LabelValue(""),
	}

	ls.RemoveEmptyLabels()

	if _, prs := ls["five"]; prs {
		t.Errorf(`key "five" not removed from map`)
	}

	if len(ls) != 2 {
		t.Errorf("expected 2 keys in labelset; found %d:\n%v\n", len(ls), ls)
	}
}
