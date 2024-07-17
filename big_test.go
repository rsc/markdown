// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"strings"
	"testing"
)

var rep = strings.Repeat

func repf(f func(int) string, n int) string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = f(i)
	}
	return strings.Join(out, "")
}

// Many cases here derived from cmark-gfm/test/pathological_tests.py

var bigTests = []struct {
	name string
	in   string
	out  string
}{
	{
		"nested strong emph",
		rep("*a **a ", 65000) + "b" + rep(" a** a*", 65000),
		"<p>" + rep("<em>a <strong>a ", 65000) + "b" + rep(" a</strong> a</em>", 65000) + "</p>\n",
	},
	{
		"many emph closers with no openers",
		rep("a_ ", 65000),
		"",
	},
	{
		"many emph openers with no closers",
		rep("_a ", 65000),
		"",
	},
	{
		"many link closers with no openers",
		rep("a]", 65000),
		"",
	},
	{
		"many link openers with no closers",
		rep("[a", 65000),
		"",
	},
	{
		"mismatched openers and closers",
		rep("*a_ ", 50000),
		"",
	},
	{
		"openers and closers multiple of 3",
		"a**b" + rep("c* ", 50000),
		"",
	},
	{
		"link openers and emph closers",
		rep("[ a_", 50000),
		"",
	},
	{
		"pattern [ (]( repeated",
		rep("[ (](", 80000),
		"",
	},
	{
		"pattern ![[]() repeated",
		rep("![[]()", 160000),
		"<p>" + rep(`![<a href=""></a>`, 160000) + "</p>\n",
	},
	{
		"hard link/emph case",
		"**x [a*b**c*](d)",
		`<p>**x <a href="d">a<em>b**c</em></a></p>` + "\n",
	},
	{
		"nested brackets",
		rep("[", 50000) + "a" + rep("]", 50000),
		"",
	},
	{
		"nested block quotes",
		rep("> ", 50000) + "a",
		rep("<blockquote>\n", 50000) + "<p>a</p>\n" + rep("</blockquote>\n", 50000),
	},
	{
		"deeply nested lists",
		repf(func(x int) string { return rep("  ", x) + "* a\n" }, 4000),
		"<ul>\n" + rep("<li>a\n<ul>\n", 4000-1) + "<li>a</li>\n" + rep("</ul>\n</li>\n", 4000-1) + "</ul>\n",
	},
	{
		"backticks",
		repf(func(x int) string { return "e" + rep("`", x) }, 5000),
		"",
	},
	{
		"backticks2",
		repf(func(x int) string { return "e" + rep("`", 5000-x) }, 5000),
		"",
	},
	{
		"unclosed links A",
		rep("[a](<b", 30000),
		"<p>" + rep("[a](&lt;b", 30000) + "</p>\n",
	},
	{
		"unclosed links B",
		rep("[a](b", 30000),
		"",
	},
	{
		"unclosed links C",
		rep("[a](b\\#", 30000),
		"<p>" + rep("[a](b#", 30000) + "</p>\n",
	},
	{
		"unclosed <!--",
		"</" + rep(" <!--", 30000),
		"<p>&lt;/" + rep(" &lt;!--", 30000) + "</p>\n",
	},
	{
		"unclosed <?",
		"</" + rep(" <?", 30000),
		"<p>&lt;/" + rep(" &lt;?", 30000) + "</p>\n",
	},
	{
		"unclosed <!X",
		"</" + rep(" <!X", 30000),
		"<p>&lt;/" + rep(" &lt;!X", 30000) + "</p>\n",
	},
	{
		"unclosed <![CDATA[",
		"</" + rep(" <![CDATA[", 30000),
		"<p>&lt;/" + rep(" &lt;![CDATA[", 30000) + "</p>\n",
	},
	{
		"tables",
		rep("abc\ndef\n|-\n", 30000),
		"<p>abc</p>\n<table>\n<thead>\n<tr>\n<th>def</th>\n</tr>\n</thead>\n<tbody>\n" +
			rep("<tr>\n<td>abc</td>\n</tr>\n<tr>\n<td>def</td>\n</tr>\n<tr>\n<td>-</td>\n</tr>\n", 30000-1) +
			"</tbody>\n</table>\n",
	},
}

func compress(s string) string {
	var out []byte
	start := 0
S:
	for i := 0; i+4 < len(s); i++ {
		c := s[i]
		for j := i + 1; j < i+100 && j < len(s); j++ {
			if s[j] == c {
				n := 1
				w := j - i
				for j+w <= len(s) && s[i:i+w] == s[j:j+w] {
					j += w
					n++
				}
				if n > 2 {
					out = append(out, s[start:i]...)
					out = fmt.Appendf(out, "«%d:%s»", n, s[i:i+w])
					start = j
					i = start - 1
					continue S
				}
			}
		}
	}
	out = append(out, s[start:]...)
	return string(out)
}

func TestBig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}
	for _, tt := range bigTests {
		t.Run(tt.name, func(t *testing.T) {
			var p Parser
			p.Table = true
			doc := p.Parse(tt.in)
			out := ToHTML(doc)
			if tt.out == "" {
				tt.out = "<p>" + strings.TrimSpace(tt.in) + "</p>\n"
			}
			if out != tt.out {
				t.Fatalf("%s: ToHTML(%q):\nhave %q\nwant %q", tt.name, compress(tt.in), compress(out), compress(tt.out))
			}
		})
	}
}

func bench(b *testing.B, text string) {
	for i := 0; i < b.N; i++ {
		var p Parser
		_ = ToHTML(p.Parse(text))
	}
	b.SetBytes(int64(len(text)))
}

func BenchmarkBrackets(b *testing.B) {
	bench(b, rep("[", 10000)+"a"+rep("]", 10000))
}

func BenchmarkDeepList(b *testing.B) {
	bench(b, repf(func(x int) string { return rep("  ", x) + "* a\n" }, 1000))
}

func BenchmarkList(b *testing.B) {
	bench(b, repf(func(x int) string { return "* a\n" }, 1000))
}
