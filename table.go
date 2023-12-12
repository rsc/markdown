// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"strings"
)

func isTableSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\v' || c == '\f'
}

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

func tableTrimOuter(row string) string {
	row = tableTrimSpace(row)
	if len(row) > 0 && row[0] == '|' {
		row = row[1:]
	}
	if len(row) > 0 && row[len(row)-1] == '|' {
		row = row[:len(row)-1]
	}
	return row
}

func isTableStart(hdr, delim string) bool {
	// Scan potential delimiter string, counting columns.
	// This happens on every line of text,
	// so make it relatively quick - nothing expensive.
	col := 0
	delim = tableTrimOuter(delim)
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
	return col == tableCount(hdr)
}

func tableCount(row string) int {
	row = tableTrimOuter(row)
	col := 1
	for i := 0; i < len(row); i++ {
		c := row[i]
		if c == '\\' {
			i++
			continue
		}
		if c == '|' {
			col++
		}
	}
	return col
}

type tableBuilder struct {
	hdr   string
	delim string
	rows  []string
}

func (b *tableBuilder) start(hdr, delim string) {
	b.hdr = tableTrimOuter(hdr)
	b.delim = tableTrimOuter(delim)
}

func (b *tableBuilder) addRow(row string) {
	b.rows = append(b.rows, tableTrimOuter(row))
}

type Table struct {
	Position
	Header []*Text
	Align  []string // 'l', 'c', 'r' for left, center, right; 0 for unset
	Rows   [][]*Text
}

func (t *Table) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<table>\n")
	buf.WriteString("<thead>\n")
	buf.WriteString("<tr>\n")
	for i, hdr := range t.Header {
		buf.WriteString("<th")
		if t.Align[i] != "" {
			buf.WriteString(" align=\"")
			buf.WriteString(t.Align[i])
			buf.WriteString("\"")
		}
		buf.WriteString(">")
		hdr.PrintHTML(buf)
		buf.WriteString("</th>\n")
	}
	buf.WriteString("</tr>\n")
	buf.WriteString("</thead>\n")
	if len(t.Rows) > 0 {
		buf.WriteString("<tbody>\n")
		for _, row := range t.Rows {
			buf.WriteString("<tr>\n")
			for i, cell := range row {
				buf.WriteString("<td")
				if i < len(t.Align) && t.Align[i] != "" {
					buf.WriteString(" align=\"")
					buf.WriteString(t.Align[i])
					buf.WriteString("\"")
				}
				buf.WriteString(">")
				cell.PrintHTML(buf)
				buf.WriteString("</td>\n")
			}
			buf.WriteString("</tr>\n")
		}
		buf.WriteString("</tbody>\n")
	}
	buf.WriteString("</table>\n")
}

func (t *Table) printMarkdown(buf *bytes.Buffer, s mdState) {
}

func (b *tableBuilder) build(p buildState) Block {
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

func (b *tableBuilder) parseRow(p buildState, row string, line int, width int) []*Text {
	out := make([]*Text, 0, width)
	pos := Position{StartLine: line, EndLine: line}
	start := 0
	unesc := nop
	for i := 0; i < len(row); i++ {
		c := row[i]
		if c == '\\' {
			i++
			if i < len(row) && row[i] == '|' {
				// Need to rewrite escaped pipe to pipe in cell.
				unesc = tableUnescape
			}
			continue
		}
		if c == '|' {
			out = append(out, p.newText(pos, unesc(strings.Trim(row[start:i], " \t\v\f"))))
			if len(out) == width {
				// Extra cells are discarded!
				return out
			}
			start = i + 1
			unesc = nop
		}
	}
	out = append(out, p.newText(pos, unesc(strings.Trim(row[start:], " \t\v\f"))))
	for len(out) < width {
		// Missing cells are considered empty.
		out = append(out, p.newText(pos, ""))
	}
	return out
}

func nop(text string) string {
	return text
}

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

func (b *tableBuilder) parseAlign(delim string, n int) []string {
	align := make([]string, 0, tableCount(delim))
	start := 0
	for i := 0; i < len(delim); i++ {
		if delim[i] == '|' {
			align = append(align, tableAlign(delim[start:i]))
			start = i + 1
		}
	}
	align = append(align, tableAlign(delim[start:]))
	return align
}

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
