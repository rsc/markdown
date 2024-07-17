// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"
)

type blockBuilder interface {
	extend(p *parser, s line) (line, bool)
	build(*parser) Block
}

type openBlock struct {
	builder blockBuilder
	inner   []Block
	pos     Position
}

func (p *parser) last() Block {
	ob := &p.stack[len(p.stack)-1]
	return ob.inner[len(ob.inner)-1]
}

func (p *parser) deleteLast() {
	ob := &p.stack[len(p.stack)-1]
	ob.inner = ob.inner[:len(ob.inner)-1]
}

type rootBuilder struct{}

func (b *rootBuilder) build(p *parser) Block {
	return &Document{p.pos(), p.blocks(), p.links}
}

// A Parser is a Markdown parser.
// The exported fields in the struct can be filled in before calling
// [Parser.Parse] in order to customize the details of the parsing process.
// A Parser is safe for concurrent use by multiple goroutines.
type Parser struct {
	// HeadingID determines whether the parser accepts
	// the {#hdr} syntax for an HTML id="hdr" attribute on headings.
	// For example, if HeadingIDs is true then the Markdown
	//    ## Overview {#overview}
	// will render as the HTML
	//    <h2 id="overview">Overview</h2>
	HeadingID bool

	// Strikethrough determines whether the parser accepts
	// ~abc~ and ~~abc~~ as strikethrough syntax, producing
	// <del>abc</del> in HTML.
	Strikethrough bool

	// TaskList determines whether the parser accepts
	// “task list items” as defined in GitHub Flavored Markdown.
	// When a list item begins with the plain text [ ] or [x]
	// that turns into an unchecked or checked check box.
	TaskList bool

	// TODO
	AutoLinkText       bool
	AutoLinkAssumeHTTP bool

	// TODO
	Table bool

	// TODO
	Emoji bool

	// TODO
	SmartDot   bool
	SmartDash  bool
	SmartQuote bool

	// TODO
	Footnote bool
}

type parser struct {
	*Parser

	corner bool // noticed corner case to ignore in cross-implementation testing

	root      *Document
	links     map[string]*Link
	lineno    int
	stack     []openBlock
	lineDepth int
	lineInfo

	// texts to apply inline processing to
	texts []textRaw

	footnotes map[string]*Footnote

	// inline parsing
	s       string
	emitted int // s[:emitted] has been emitted into list
	list    []Inline

	backticks backtickParser

	fixups []func()
}

func (p *parser) addFixup(f func()) {
	p.fixups = append(p.fixups, f)
}

type lineInfo struct {
	noDeclEnd     bool // no > on line
	noCommentEnd  bool // no --> on line
	noProcInstEnd bool // no ?> on line
	noCDATAEnd    bool // ]]> on line
}

type textRaw struct {
	*Text
	raw string
}

func (p *parser) newText(pos Position, text string) *Text {
	b := &Text{Position: pos}
	p.texts = append(p.texts, textRaw{b, text})
	return b
}

func (p *parser) blocks() []Block {
	b := &p.stack[len(p.stack)-1]
	return b.inner
}

func (p *parser) pos() Position {
	b := &p.stack[len(p.stack)-1]
	return b.pos
}

func (p *Parser) Parse(text string) *Document {
	d, _ := p.parse(text)
	return d
}

func (p *Parser) parse(text string) (d *Document, corner bool) {
	var ps parser
	ps.Parser = p
	if strings.Contains(text, "\x00") {
		text = strings.ReplaceAll(text, "\x00", "\uFFFD")
		ps.corner = true // goldmark does not replace NUL
	}

	ps.lineDepth = -1
	ps.addBlock(&rootBuilder{})
	for text != "" {
		end := 0
		for end < len(text) && text[end] != '\n' && text[end] != '\r' {
			end++
		}
		ln := text[:end]
		text = text[end:]
		nl := byte(0)
		switch {
		case len(text) >= 2 && text[0] == '\r' && text[1] == '\n':
			nl = '\r' + '\n'
			text = text[2:]
		case len(text) >= 1:
			nl = text[0]
			text = text[1:]
		}
		ps.lineno++
		ps.addLine(makeLine(ln, nl))
	}
	ps.trimStack(0)

	for _, t := range ps.texts {
		t.Inline = ps.inline(t.raw)
	}

	for _, f := range ps.fixups {
		f()
	}

	// TODO move into its own function
	var fixBlock func(Block)

	fixBlocks := func(blocks []Block) []Block {
		keep := blocks[:0]
		for _, b := range blocks {
			fixBlock(b)
			if _, ok := b.(*Empty); ok {
				continue
			}
			keep = append(keep, b)
		}
		return keep
	}

	fixBlock = func(x Block) {
		switch x := x.(type) {
		case *Document:
			x.Blocks = fixBlocks(x.Blocks)
		case *Quote:
			x.Blocks = fixBlocks(x.Blocks)
		case *List:
			for _, item := range x.Items {
				fixBlock(item)
			}
		case *Item:
			x.Blocks = fixBlocks(x.Blocks)
		}
	}

	fixBlock(ps.root)

	return ps.root, ps.corner
}

func (p *parser) curB() blockBuilder {
	if p.lineDepth < len(p.stack) {
		return p.stack[p.lineDepth].builder
	}
	return nil
}

func (p *parser) nextB() blockBuilder {
	if p.lineDepth+1 < len(p.stack) {
		return p.stack[p.lineDepth+1].builder
	}
	return nil
}
func (p *parser) trimStack(depth int) {
	if len(p.stack) < depth {
		// unreachable
		panic("trimStack")
	}
	for len(p.stack) > depth {
		p.closeBlock()
	}
}

func (p *parser) addBlock(c blockBuilder) {
	p.trimStack(p.lineDepth + 1)
	p.stack = append(p.stack, openBlock{})
	ob := &p.stack[len(p.stack)-1]
	ob.builder = c
	ob.pos.StartLine = p.lineno
	ob.pos.EndLine = p.lineno
}

func (p *parser) doneBlock(b Block) {
	p.trimStack(p.lineDepth + 1)
	ob := &p.stack[len(p.stack)-1]
	ob.inner = append(ob.inner, b)
}

func (p *parser) para() *paraBuilder {
	if b, ok := p.stack[len(p.stack)-1].builder.(*paraBuilder); ok {
		return b
	}
	return nil
}

func (p *parser) closeBlock() Block {
	b := &p.stack[len(p.stack)-1]
	if b.builder == nil {
		println("closeBlock", len(p.stack)-1)
	}
	blk := b.builder.build(p)
	p.stack = p.stack[:len(p.stack)-1]
	if len(p.stack) > 0 {
		b := &p.stack[len(p.stack)-1]
		b.inner = append(b.inner, blk)
		// _ = b
	} else {
		p.root = blk.(*Document)
	}
	return blk
}

func (p *parser) link(label string) *Link {
	return p.links[label]
}

func (p *parser) defineLink(label string, link *Link) {
	if p.links == nil {
		p.links = make(map[string]*Link)
	}
	p.links[label] = link
}

func (p *parser) addLine(s line) {
	// Process continued prefixes.
	p.lineDepth = 0
	for ; p.lineDepth+1 < len(p.stack); p.lineDepth++ {
		old := s
		var ok bool
		s, ok = p.stack[p.lineDepth+1].builder.extend(p, s)
		// Note: s != old is efficient only because s.text is either the same string (same pointer, len)
		// as old.text or has a different length or is empty; either way so there is no actual data comparison.
		// Sometimes s.text = "" and there is still
		if (ok || s != old) && !old.isBlank() {
			p.stack[p.lineDepth+1].pos.EndLine = p.lineno
		}
		if !ok {
			break
		}
	}

	if s.isBlank() {
		p.trimStack(p.lineDepth + 1)
		return
	}

	// Process new prefixes, if any.
Prefixes:
	// Start new block inside p.stack[depth].
	for _, fn := range starters {
		if l, ok := fn(p, s); ok {
			s = l
			if s.isBlank() {
				return
			}
			p.lineDepth++
			goto Prefixes
		}
	}

	startParagraph(p, s)
}

func (c *rootBuilder) extend(p *parser, s line) (line, bool) {
	// unreachable
	panic("root extend")
}

type starter func(*parser, line) (line, bool)

var starters = []starter{
	startIndentedCodeBlock,
	startFencedCodeBlock,
	startBlockQuote,
	startATXHeading,
	startSetextHeading,
	startThematicBreak,
	startListItem,
	startHTMLBlock,
	startFootnote,
}
