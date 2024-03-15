// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import "testing"

var tableCountTests = []struct {
	row string
	n   int
}{
	{"|", 1},
	{"|x|", 1},
	{"||", 1},
	{"| |", 1},
	{"| | |", 2},
	{"| | Foo | Bar |", 3},
	{"|          | Foo      | Bar      |", 3},
	{"", 1},
	{"|a|b", 2},
	{"|a| ", 1},
	{" |b", 1},
	{"a|b", 2},
	{`x\|y`, 1},
	{`x\\|y`, 1},
	{`x\\\|y`, 1},
	{`x\\\\|y`, 1},
	{`x\\\\\|y`, 1},
	{`| 0\|1\\|2\\\|3\\\\|4\\\\\|5\\\\\\|6\\\\\\\|7\\\\\\\\|8  |`, 1},
}

func TestTableCount(t *testing.T) {
	for _, tt := range tableCountTests {
		n := tableCount(tableTrimOuter(tt.row))
		if n != tt.n {
			t.Errorf("tableCount(%#q) = %d, want %d", tt.row, n, tt.n)
		}
	}
}

func TestTablePaddedCell(t *testing.T) {
	testCases := []struct {
		raw, align string
		w          int

		want string
	}{
		{"foo", "center", 8, "  foo   "},
		{"foo", "center", 6, " foo  "},
		{"foo", "center", 5, " foo "},
		{"foo", "center", 4, "foo "},
		{"foo", "center", 3, "foo"},

		{"foo", "left", 8, "foo     "},
		{"foo", "right", 8, "     foo"},
		{"foo", "", 8, "foo     "},

		{"foo", "left", 6, "foo   "},
		{"foo", "right", 6, "   foo"},
		{"foo", "", 6, "foo   "},

		{"foo", "left", 5, "foo  "},
		{"foo", "right", 5, "  foo"},
		{"foo", "", 5, "foo  "},

		{"foo", "left", 4, "foo "},
		{"foo", "right", 4, " foo"},
		{"foo", "", 4, "foo "},

		{"foo", "left", 3, "foo"},
		{"foo", "right", 3, "foo"},
		{"foo", "", 3, "foo"},
	}

	for _, tc := range testCases {
		in := tc.raw
		a := tc.align
		w := tc.w
		want := tc.want
		if h := paddedCell(in, a, w); h != want {
			t.Errorf("\npad(%s, %s, %d)\n have %q\n want %q", in, a, w, h, want)
		}
	}
}
