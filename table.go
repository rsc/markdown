// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"
	"unicode/utf8"
)

// A Table is a [Block] representing a [table], a GitHub-flavored Markdown extension.
//
// [table]: https://github.github.com/gfm/#tables-extension-
type Table struct {
	Position
	Header []*Text   // header row (slice of columns)
	Align  []string  // alignment for columns: "left", "center", "right"; "" for unset
	Rows   [][]*Text // data rows (slices of columns, not necessarily all same width)
}

func (*Table) Block() {}

func (t *Table) printHTML(p *printer) {
	p.html("<table>\n")
	p.html("<thead>\n")
	p.html("<tr>\n")
	for i, hdr := range t.Header {
		p.html("<th")
		if t.Align[i] != "" {
			p.html(` align="`, t.Align[i], `"`)
		}
		p.html(">")
		hdr.printHTML(p)
		p.html("</th>\n")
	}
	p.html("</tr>\n")
	p.html("</thead>\n")
	if len(t.Rows) > 0 {
		p.html("<tbody>\n")
		for _, row := range t.Rows {
			p.html("<tr>\n")
			for i, cell := range row {
				p.html("<td")
				if i < len(t.Align) && t.Align[i] != "" {
					p.html(` align="`, t.Align[i], `"`)
				}
				p.html(">")
				cell.printHTML(p)
				p.html("</td>\n")
			}
			p.html("</tr>\n")
		}
		p.html("</tbody>\n")
	}
	p.html("</table>\n")
}

func (t *Table) printMarkdown(p *printer) {
	// TODO: double-check this
	// inline all Text values in Header and Rows to
	// get final, rendered widths
	var (
		hdr       = make([]string, len(t.Header))
		rows      = make([][]string, 0, len(t.Rows))
		maxWidths = make([]int, len(t.Header))

		xb = &printer{}
		xs string
	)

	toString := func(txt *Text) string {
		xb.buf.Reset()
		txt.printMarkdown(xb)
		return strings.TrimSpace(xb.buf.String())
	}

	for i, txt := range t.Header {
		xs = toString(txt)
		hdr[i] = xs
		maxWidths[i] = utf8.RuneCountInString(xs)
	}

	for _, row := range t.Rows {
		xrow := make([]string, len(hdr))
		for j := range t.Header {
			xs = toString(row[j])
			xrow[j] = xs
			if n := utf8.RuneCountInString(xs); n > maxWidths[j] {
				maxWidths[j] = n
			}
		}
		rows = append(rows, xrow)
	}

	p.maybeQuoteNL('|')
	for i, cell := range hdr {
		p.WriteString("| ")
		pad(p, cell, t.Align[i], maxWidths[i])
		p.WriteString(" ")
	}
	p.WriteString("|")

	p.nl()
	for i, a := range t.Align {
		w := maxWidths[i]
		p.WriteString("| ")
		switch a {
		case "left":
			p.WriteString(":")
			repeat(p, '-', w-1)
		case "center":
			p.WriteString(":")
			repeat(p, '-', w-2)
			p.WriteString(":")
		case "right":
			repeat(p, '-', w-1)
			p.WriteString(":")
		default:
			repeat(p, '-', w)
		}
		p.WriteString(" ")
	}
	p.WriteString("|")

	for _, row := range rows {
		p.nl()
		for i := range t.Header {
			p.WriteString("| ")
			pad(p, row[i], t.Align[i], maxWidths[i])
			p.WriteString(" ")
		}
		p.WriteString("|")
	}
}

// repeat prints c n times to p.
func repeat(p *printer, c byte, n int) {
	for i := 0; i < n; i++ {
		p.WriteByte(c)
	}
}

// pad prints text to p aligned according to align,
// aiming for a width of w runes.
// It can happen that multiple runes appear as a single “character”,
// which will break the alignment, but this is the best we can do for now.
func pad(p *printer, text, align string, w int) {
	n := w - utf8.RuneCountInString(text)
	switch align {
	default:
		p.WriteString(text)
		repeat(p, ' ', n)
	case "right":
		repeat(p, ' ', n)
		p.WriteString(text)
	case "center":
		repeat(p, ' ', n/2)
		p.WriteString(text)
		repeat(p, ' ', n-n/2)
	}
}

// A tableTrimmed is a table row with the outer pipes (if any) removed.
// It is a separate type to avoid accidentally trimming the outer pipes multiple times,
// which would instead discard outer empty cells.
type tableTrimmed string

// isTableSpace reports whether c is a space as far as tables are concerned.
func isTableSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f'
}

// tableTrimSpace returns s with table space prefixes and suffixes removed.
func tableTrimSpace(s string) string {
	i := 0
	for i < len(s) && isTableSpace(s[i]) {
		i++
	}
	j := len(s)
	for j > i && isTableSpace(s[j-1]) {
		j--
	}
	return s[i:j]
}

// tableTrimOuter trims the outer | |, if any, from the row.
func tableTrimOuter(row string) tableTrimmed {
	row = tableTrimSpace(row)
	if len(row) > 0 && row[0] == '|' {
		row = row[1:]
	}
	if len(row) > 0 && row[len(row)-1] == '|' {
		row = row[:len(row)-1]
	}
	return tableTrimmed(row)
}

// isTableStart reports whether the pair of lines hdr1, delim1
// are a valid table start.
func isTableStart(hdr1, delim1 string) bool {
	// Scan potential delimiter string, counting columns.
	// This happens on every line of text,
	// so make it relatively quick - nothing expensive.
	col := 0
	delim := tableTrimOuter(delim1)
	i := 0
	for ; ; col++ {
		for i < len(delim) && isTableSpace(delim[i]) {
			i++
		}
		if i >= len(delim) {
			break
		}
		if i < len(delim) && delim[i] == ':' {
			i++
		}
		if i >= len(delim) || delim[i] != '-' {
			return false
		}
		i++
		for i < len(delim) && delim[i] == '-' {
			i++
		}
		if i < len(delim) && delim[i] == ':' {
			i++
		}
		for i < len(delim) && isTableSpace(delim[i]) {
			i++
		}
		if i < len(delim) && delim[i] == '|' {
			i++
		}
	}

	if tableTrimSpace(hdr1) == "|" {
		// https://github.com/github/cmark-gfm/pull/127 and
		// https://github.com/github/cmark-gfm/pull/128
		// fixed a buffer overread by rejecting | by itself as a table line.
		// That seems to violate the “spec”, but we will play along.
		return false
	}

	return col == tableCount(tableTrimOuter(hdr1))
}

// tableCount returns the number of columns in the row.
func tableCount(row tableTrimmed) int {
	col := 1
	prev := byte(0)
	for i := 0; i < len(row); i++ {
		c := row[i]
		if c == '|' && prev != '\\' {
			col++
		}
		prev = c
	}
	return col
}

// A tableBuilder is a [blockBuilder] for a [Table].
type tableBuilder struct {
	hdr   tableTrimmed   // header line
	delim tableTrimmed   // delimiter line
	rows  []tableTrimmed // data lines
}

// start starts the builder with the given header and delimiter lines.
func (b *tableBuilder) start(hdr, delim string) {
	b.hdr = tableTrimOuter(hdr)
	b.delim = tableTrimOuter(delim)
}

// addRow adds a new row to the table.
func (b *tableBuilder) addRow(row string) {
	b.rows = append(b.rows, tableTrimOuter(row))
}

// build returns the [Table] for this tableBuilder.
func (b *tableBuilder) build(p *parser) Block {
	pos := p.pos()
	pos.StartLine-- // builder does not count header
	pos.EndLine = pos.StartLine + 1 + len(b.rows)
	t := &Table{
		Position: pos,
	}
	width := tableCount(b.hdr)
	t.Header = b.parseRow(p, b.hdr, pos.StartLine, width)
	t.Align = b.parseAlign(b.delim, width)
	t.Rows = make([][]*Text, len(b.rows))
	for i, row := range b.rows {
		t.Rows[i] = b.parseRow(p, row, pos.StartLine+2+i, width)
	}
	return t
}

// parseRow TODO explain
func (b *tableBuilder) parseRow(p *parser, row tableTrimmed, line int, width int) []*Text {
	out := make([]*Text, 0, width)
	pos := Position{StartLine: line, EndLine: line}
	start := 0
	unesc := nop
	for i := 0; i < len(row); i++ {
		c := row[i]
		if c == '\\' && i+1 < len(row) && row[i+1] == '|' {
			unesc = tableUnescape
			i++
			continue
		}
		if c == '|' {
			out = append(out, p.newText(pos, unesc(strings.Trim(string(row[start:i]), " \t\v\f"))))
			if len(out) == width {
				// Extra cells are discarded!
				return out
			}
			start = i + 1
			unesc = nop
		}
	}
	out = append(out, p.newText(pos, unesc(strings.Trim(string(row[start:]), " \t\v\f"))))
	for len(out) < width {
		// Missing cells are considered empty.
		out = append(out, p.newText(pos, ""))
	}
	return out
}

func nop(text string) string {
	return text
}

// tableUnescape TODO
func tableUnescape(text string) string {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '\\' && i+1 < len(text) && text[i+1] == '|' {
			i++
			c = '|'
		}
		out = append(out, c)
	}
	return string(out)
}

// parseAlign TODO
func (b *tableBuilder) parseAlign(delim tableTrimmed, n int) []string {
	align := make([]string, 0, tableCount(delim))
	start := 0
	for i := 0; i < len(delim); i++ {
		if delim[i] == '|' {
			align = append(align, tableAlign(string(delim[start:i])))
			start = i + 1
		}
	}
	align = append(align, tableAlign(string(delim[start:])))
	return align
}

// tableAlign TODO
func tableAlign(cell string) string {
	cell = tableTrimSpace(cell)
	l := cell[0] == ':'
	r := cell[len(cell)-1] == ':'
	switch {
	case l && r:
		return "center"
	case l:
		return "left"
	case r:
		return "right"
	}
	return ""
}
