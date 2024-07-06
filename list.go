// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"strconv"
)

// TODO should Item implement Block?
// maybe make a itemBlock internal Block for use with the builders?

// A List is a [Block] representing a [list],
// either an unordered (bullet) list
// or an ordered (numbered) list.
//
// Lists can be [loose or tight], which controls the spacing between list items.
// In Markdown, a list is loose when there is a blank line
// between any two list items, or when any list item
// directly contains two blocks that are separated by a blank line.
// (Note that because paragraphs must be separated by blank lines,
// any multi-paragraph item necessarily creates a loose list.)
// When rendering HTML, loose list items are formatted in the usual way.
// For tight lists, a list item consisting of a single paragraph omits
// the <p>...</p> tags around the paragraph text.
//
// [list]: https://spec.commonmark.org/0.31.2/#lists
// [loose or tight]: https://spec.commonmark.org/0.31.2/#loose
type List struct {
	Position

	// Bullet is the bullet character used in the list: '-', '+', or '*'.
	// For an ordered list, Bullet is the character following the number: '.' or ')'.
	Bullet rune

	// Start is the number of the first item in an ordered list.
	Start int

	// Loose indicates whether the list is loose.
	// (See the [List] doc comment for details.)
	Loose bool

	// Items is the list's items.
	// TODO: Should this be []*Item or Blocks?
	Items []Block // always *Item
}

func (*List) Block() {}

// Ordered reports whether the list is ordered (numbered).
func (l *List) Ordered() bool {
	return l.Bullet == '.' || l.Bullet == ')'
}

// An Item is a [Block] representing a [list item].
//
// [list item]: https://spec.commonmark.org/0.31.2/#list-items
type Item struct {
	Position

	// Blocks is the item content.
	Blocks []Block
}

func (*Item) Block() {}

func (b *List) printHTML(p *printer) {
	if b.Bullet == '.' || b.Bullet == ')' {
		p.html("<ol")
		if b.Start != 1 {
			p.html(` start="`, strconv.Itoa(b.Start), `"`)
		}
		p.html(">\n")
	} else {
		p.html("<ul>\n")
	}
	for _, item := range b.Items {
		item.printHTML(p)
	}
	if b.Bullet == '.' || b.Bullet == ')' {
		p.html("</ol>\n")
	} else {
		p.html("</ul>\n")
	}
}

func (b *Item) printHTML(p *printer) {
	p.html("<li>")
	if len(b.Blocks) > 0 {
		if _, ok := b.Blocks[0].(*Text); !ok {
			p.WriteString("\n")
		}
	}
	for i, c := range b.Blocks {
		c.printHTML(p)
		if i+1 < len(b.Blocks) {
			if _, ok := c.(*Text); ok {
				p.WriteString("\n")
			}
		}
	}
	p.html("</li>\n")
}

func (b *List) printMarkdown(p *printer) {
	old := p.listOut
	defer func() {
		p.listOut = old
	}()
	p.bullet = b.Bullet
	p.num = b.Start
	if b.Loose {
		p.loose++
	} else {
		p.tight++
	}
	p.maybeNL()
	for i, item := range b.Items {
		if i > 0 {
			p.nl()
			if b.Loose {
				p.nl()
			}
		}
		item.printMarkdown(p)
		p.num++
	}
}

func (b *Item) printMarkdown(p *printer) {
	var marker string
	if p.bullet == '.' || p.bullet == ')' {
		marker = fmt.Sprintf(" %d%c ", p.num, p.bullet)
	} else {
		marker = fmt.Sprintf("  %c ", p.bullet)
	}
	p.WriteString(marker)
	n := len(marker)
	if n > 4 {
		n = 4
	}
	defer p.pop(p.push("    "[:n]))
	printMarkdownBlocks(b.Blocks, p)
}

// A listBuilder is a [blockBuilder] for a [List].
type listBuilder struct {
	// List fields
	bullet rune
	start  int

	// item is the builder for the current item.
	item *itemBuilder

	//
	todo func() line
}

// An itemBuilder is a [blockBuilder] for an [Item].
type itemBuilder struct {
	list        *listBuilder //  list containing item
	width       int          // TODO
	haveContent bool         // TODO
}

// TODO explain
// startListItem is a [starter] for a list item.
// The first list item in a list also starts the list itself.
func startListItem(p *parser, s line) (_ line, _ bool) {
	if list, ok := p.curB().(*listBuilder); ok && list.todo != nil {
		s = list.todo()
		list.todo = nil
		return s, true
	}

	t := s
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
		return
	case '-', '*', '+':
		t.trim(bullet)
		n++
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		for j := t.i; ; j++ {
			if j >= len(t.text) {
				return
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
				return
			}
			if j-t.i >= 9 {
				return
			}
			num = num*10 + int(c) - '0'
		}

	}
	if !t.trimSpace(1, 1, true) {
		return
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

	// Pretty sure we have a list item now.

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
		if list == nil && p.para() != nil && (t.isBlank() || (bullet == '.' || bullet == ')') && num != 1) {
			// Goldmark and Dingus both seem to get this wrong
			// (or the words above don't mean what we think they do).
			// when the paragraph that could be continued
			// is inside a block quote.
			// See testdata/extra.txt 117.md.
			p.corner = true
			return
		}
		list = &listBuilder{bullet: rune(bullet), start: num}
		p.addBlock(list)
	}
	b := &itemBuilder{list: list, width: n, haveContent: !t.isBlank()}
	list.todo = func() line {
		p.addBlock(b)
		list.item = b
		return t
	}

	// TODO explain s not t
	return s, true
}

func (c *listBuilder) extend(p *parser, s line) (line, bool) {
	// TODO explain
	item := c.item
	if item == nil && s.isBlank() { // TODO how can this happen
		return s, true
	}

	// If we can trim the indentation required by the current item,
	// do that and return true, allowing s to be passed to the
	// item builder.
	if item != nil && s.trimSpace(item.width, item.width, true) {
		return s, true
	}
	return s, false
}

func (c *itemBuilder) extend(p *parser, s line) (line, bool) {
	blank := s.isBlank()

	// If there is a blank line and no content so far,
	// the item is over. TODO explain
	if blank && !c.haveContent {
		return s, false
	}

	// TODO explain
	if blank {
		// Goldmark does this and apparently commonmark.js too.
		// Not sure why it is necessary.
		return line{}, true
	}

	// TODO explain
	if !blank {
		c.haveContent = true
	}
	return s, true
}

func (b *itemBuilder) build(p *parser) Block {
	b.list.item = nil
	return &Item{p.pos(), p.blocks()}
}

func (b *listBuilder) build(p *parser) Block {
	blocks := p.blocks()
	pos := p.pos()

	// list can have wrong pos b/c extend dance.
	// TODO explain
	pos.EndLine = blocks[len(blocks)-1].Pos().EndLine

	// Decide whether list is loose.
	loose := false
Loose:
	for i, c := range blocks {
		c := c.(*Item)
		if i+1 < len(blocks) {
			if blocks[i+1].Pos().StartLine-c.EndLine > 1 {
				loose = true
				break Loose
			}
		}
		for j, d := range c.Blocks {
			endLine := d.Pos().EndLine
			if j+1 < len(c.Blocks) {
				if c.Blocks[j+1].Pos().StartLine-endLine > 1 {
					loose = true
					break Loose
				}
			}
		}
	}

	if !loose {
		// TODO: rethink whether this is correct.
		// Perhaps the blocks should still be Paragraph
		// and we just skip over the <p> during formatting?
		// Then Text might not need to be a Block.
		for _, c := range blocks {
			c := c.(*Item)
			for i, d := range c.Blocks {
				if p, ok := d.(*Paragraph); ok {
					c.Blocks[i] = p.Text
				}
			}
		}
	}

	x := &List{
		pos,
		b.bullet,
		b.start,
		loose,
		p.blocks(),
	}
	listCorner(p, x)
	if p.TaskList {
		p.addFixup(func() {
			parseTaskList(p, x)
		})
	}
	return x
}

// listCorner checks whether list contains any corner cases
// that other implementations mishandle, and if so sets p.corner.
func listCorner(p *parser, list *List) {
	for _, item := range list.Items {
		item := item.(*Item)
		if len(item.Blocks) == 0 {
			// Goldmark mishandles what follows; see testdata/extra.txt 111.md.
			p.corner = true
			return
		}
		switch item.Blocks[0].(type) {
		case *List, *ThematicBreak, *CodeBlock:
			// Goldmark mishandles a list with various block items inside it.
			p.corner = true
			return
		}
	}
}

// GitHub task list extension

// A Task is an [Inline] for a [task list item marker] (a checkbox),
// a GitHub-flavored Markdown extension.
//
// [task list item marker]: https://github.github.com/gfm/#task-list-items-extension-
type Task struct {
	Checked bool
}

func (*Task) Inline() {}

func (x *Task) printHTML(p *printer) {
	p.html("<input ")
	if x.Checked {
		p.html(`checked="" `)
	}
	p.html(`disabled="" type="checkbox"> `)
}

func (x *Task) printMarkdown(p *printer) {
	if x.Checked {
		p.text(`[x] `)
	} else {
		p.text(`[ ] `)
	}
}

func (x *Task) printText(p *printer) {
	// Unreachable: printText is only used to render the
	// alt text of an image, which can only contain inlines,
	// and while Task is an inline, it only appears inside
	// lists, and a list cannot appear in an alt text.
	// Even so, maybe someone will make malformed syntax trees.
	x.printMarkdown(p)
}

// taskList checks whether any items in list begin with task list markers.
// If so, it replaces the markers with [Task]s.
func parseTaskList(p *parser, list *List) {
	for _, item := range list.Items {
		item := item.(*Item)
		if len(item.Blocks) == 0 {
			continue
		}
		var text *Text
		switch b := item.Blocks[0].(type) {
		default:
			continue
		case *Paragraph:
			text = b.Text
		case *Text:
			text = b
		}
		if len(text.Inline) < 1 {
			// unreachable with standard parser
			continue
		}
		pl, ok := text.Inline[0].(*Plain)
		if !ok {
			continue
		}
		s := pl.Text
		if len(s) < 4 || s[0] != '[' || s[2] != ']' || (s[1] != ' ' && s[1] != 'x' && s[1] != 'X') {
			continue
		}
		if s[3] != ' ' && s[3] != '\t' {
			p.corner = true // goldmark does not require the space
			continue
		}
		text.Inline = append([]Inline{&Task{Checked: s[1] == 'x' || s[1] == 'X'},
			&Plain{Text: s[len("[x] "):]}}, text.Inline[1:]...)
	}
}
