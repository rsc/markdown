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
		n := tableCount(tt.row)
		if n != tt.n {
			t.Errorf("tableCount(%#q) = %d, want %d", tt.row, n, tt.n)
		}
	}
}
