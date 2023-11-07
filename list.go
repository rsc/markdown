// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
)

type List struct {
	Position
	Bullet rune
	Start  int
	Loose  bool
	Items  []Block
}

type Item struct {
	Position
	Blocks []Block
}

func (b *List) PrintHTML(buf *bytes.Buffer) {
	if b.Bullet == '.' || b.Bullet == ')' {
		buf.WriteString("<ol")
		if b.Start != 1 {
			fmt.Fprintf(buf, " start=\"%d\"", b.Start)
		}
		buf.WriteString(">\n")
	} else {
		buf.WriteString("<ul>\n")
	}
	for _, c := range b.Items {
		c.PrintHTML(buf)
	}
	if b.Bullet == '.' || b.Bullet == ')' {
		buf.WriteString("</ol>\n")
	} else {
		buf.WriteString("</ul>\n")
	}
}

func (b *Item) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<li>")
	if len(b.Blocks) > 0 {
		if _, ok := b.Blocks[0].(*Text); !ok {
			buf.WriteString("\n")
		}
	}
	for i, c := range b.Blocks {
		c.PrintHTML(buf)
		if i+1 < len(b.Blocks) {
			if _, ok := c.(*Text); ok {
				buf.WriteString("\n")
			}
		}
	}
	buf.WriteString("</li>\n")
}

type listBuilder struct {
	bullet rune
	num    int
	loose  bool
	item   *itemBuilder
	todo   func() line
}

func (b *listBuilder) build(p buildState) Block {
	blocks := p.blocks()
	pos := p.pos()

	// list can have wrong pos b/c extend dance.
	pos.EndLine = blocks[len(blocks)-1].Pos().EndLine
Loose:
	for i, c := range blocks {
		c := c.(*Item)
		if i+1 < len(blocks) {
			if blocks[i+1].Pos().StartLine-c.EndLine > 1 {
				b.loose = true
				break Loose
			}
		}
		for j, d := range c.Blocks {
			endLine := d.Pos().EndLine
			if j+1 < len(c.Blocks) {
				if c.Blocks[j+1].Pos().StartLine-endLine > 1 {
					b.loose = true
					break Loose
				}
			}
		}
	}

	if !b.loose {
		for _, c := range blocks {
			c := c.(*Item)
			for i, d := range c.Blocks {
				if p, ok := d.(*Paragraph); ok {
					c.Blocks[i] = p.Text
				}
			}
		}
	}

	return &List{
		pos,
		b.bullet,
		b.num,
		b.loose,
		p.blocks(),
	}
}

func (b *itemBuilder) build(p buildState) Block {
	b.list.item = nil
	return &Item{p.pos(), p.blocks()}
}

func (c *listBuilder) extend(p *parser, s line) (line, bool) {
	d := c.item
	if d != nil && s.trimSpace(d.width, d.width, true) || d == nil && s.isBlank() {
		return s, true
	}
	return s, false
}

func (c *itemBuilder) extend(p *parser, s line) (line, bool) {
	if s.isBlank() && !c.haveContent {
		return s, false
	}
	if s.isBlank() {
		// Goldmark does this and apparently commonmark.js too.
		// Not sure why it is necessary.
		return line{}, true
	}
	if !s.isBlank() {
		c.haveContent = true
	}
	return s, true
}

func newListItem(p *parser, s line) (line, bool) {
	if list, ok := p.curB().(*listBuilder); ok && list.todo != nil {
		s = list.todo()
		list.todo = nil
		return s, true
	}
	if p.startListItem(&s) {
		return s, true
	}
	return s, false
}

func (p *parser) startListItem(s *line) bool {
	t := *s
	n := 0
	for i := 0; i < 3; i++ {
		if !t.trimSpace(1, 1, false) {
			break
		}
		n++
	}
	bullet := t.peek()
	var num int
Switch:
	switch bullet {
	default:
		return false
	case '-', '*', '+':
		t.trim(bullet)
		n++
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		for j := t.i; ; j++ {
			if j >= len(t.text) {
				return false
			}
			c := t.text[j]
			if c == '.' || c == ')' {
				// success
				bullet = c
				j++
				n += j - t.i
				t.i = j
				break Switch
			}
			if c < '0' || '9' < c {
				return false
			}
			if j-t.i >= 9 {
				return false
			}
			num = num*10 + int(c) - '0'
		}

	}
	if !t.trimSpace(1, 1, true) {
		return false
	}
	n++
	tt := t
	m := 0
	for i := 0; i < 3 && tt.trimSpace(1, 1, false); i++ {
		m++
	}
	if !tt.trimSpace(1, 1, true) {
		n += m
		t = tt
	}

	// point of no return

	var list *listBuilder
	if c, ok := p.nextB().(*listBuilder); ok {
		list = c
	}
	if list == nil || list.bullet != rune(bullet) {
		// “When the first list item in a list interrupts a paragraph—that is,
		// when it starts on a line that would otherwise count as
		// paragraph continuation text—then (a) the lines Ls must
		// not begin with a blank line,
		// and (b) if the list item is ordered, the start number must be 1.”
		if list == nil && p.para() != nil && (t.isBlank() || num > 1) {
			return false
		}
		list = &listBuilder{bullet: rune(bullet), num: num}
		p.addBlock(list)
	}
	b := &itemBuilder{list: list, width: n, haveContent: !t.isBlank()}
	list.todo = func() line {
		p.addBlock(b)
		list.item = b
		return t
	}
	return true
}
