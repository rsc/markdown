// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"strings"
)

/*

list block itself does not appear on stack?
item does
end of item returns block,
new item continues previous block if possible?

if close leaves lines or blocks behind, panic

close(b a list item, parent)
	if b's parent's last block is list && item can be added to it, do so
	else return new list

or maybe not parent but just current list of blocks

preserve LinkRefDefs?

*/

type markOut struct {
	bytes.Buffer
	prefix      []byte
	prefixOld   []byte
	prefixOlder []byte
	loose       int
	tight       int
	trimLimit   int
}

func (b *markOut) noTrim() {
	b.trimLimit = len(b.Bytes())
}

func (b *markOut) NL() {
	text := b.Bytes()
	for len(text) > b.trimLimit && text[len(text)-1] == ' ' {
		text = text[:len(text)-1]
	}
	b.Truncate(len(text))

	b.Buffer.WriteByte('\n')
	b.Buffer.Write(b.prefix)
	b.prefixOlder, b.prefixOld = b.prefixOld, b.prefix
}

func (b *markOut) maybeNL() bool {
	// Starting a new block that may need a blank line before it
	// to avoid being mixed into a previous block
	// as paragraph continuation text.
	//
	// If the prefix on the current line (all of cur)
	// is the same as the current continuation prefix
	// (not first line of a list item)
	// and the previous line started with the same prefix,
	// then we need a blank line to avoid looking like
	// paragraph continuation text.
	before, cur := cutLastNL(b.Bytes())
	before, prev := cutLastNL(before)
	if b.Len() > 0 && bytes.Equal(cur, b.prefix) && bytes.HasPrefix(prev, b.prefix) {
		b.NL()
		return true
	}
	return true
}

func cutLastNL(text []byte) (prefix, last []byte) {
	i := bytes.LastIndexByte(text, '\n')
	if i < 0 {
		return nil, text
	}
	return text[:i], text[i+1:]
}

func (b *markOut) maybeQuoteNL(quote byte) bool {
	// Starting a new quote block.
	// Make sure it doesn't look like it is part of a preceding quote block.
	before, cur := cutLastNL(b.Bytes())
	before, prev := cutLastNL(before)
	if len(prev) >= len(cur)+1 && bytes.HasPrefix(prev, cur) && prev[len(cur)] == quote {
		b.NL()
		return true
	}
	return false
}

func (b *markOut) WriteByte(c byte) error {
	if c == '\n' {
		panic("Write \\n")
	}
	return b.Buffer.WriteByte(c)
}

func (b *markOut) Write(p []byte) (int, error) {
	for i := range p {
		if p[i] == '\n' {
			panic("Write \\n")
		}
	}
	return b.Buffer.Write(p)
}

func (b *markOut) WriteString(s string) (int, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			panic("Write \\n")
		}
	}
	return b.Buffer.WriteString(s)
}

func (b *markOut) push(s string) int {
	n := len(b.prefix)
	b.prefix = append(b.prefix, s...)
	return n
}

func (b *markOut) pop(n int) {
	b.prefix = b.prefix[:n]
}

// Block is implemented by:
//
//	CodeBlock
//	Document
//	Empty
//	HTMLBlock
//	Heading
//	Item
//	List
//	Paragraph
//	Quote
//	Text
//	ThematicBreak
type Block interface {
	Pos() Position
	PrintHTML(buf *bytes.Buffer)
	printMarkdown(buf *markOut, s mdState)
}

type mdState struct {
	bullet rune // for list items
	num    int  // for numbered list items
}

type Position struct {
	StartLine int
	EndLine   int
}

func (p Position) Pos() Position {
	return p
}

type buildState interface {
	blocks() []Block
	pos() Position
	last() Block
	deleteLast()

	link(label string) *Link
	defineLink(label string, link *Link)
	newText(pos Position, text string) *Text
}

type blockBuilder interface {
	extend(p *parseState, s line) (line, bool)
	build(buildState) Block
}

type openBlock struct {
	builder blockBuilder
	inner   []Block
	pos     Position
}

type itemBuilder struct {
	list        *listBuilder
	width       int
	haveContent bool
}

func (p *parseState) last() Block {
	ob := &p.stack[len(p.stack)-1]
	return ob.inner[len(ob.inner)-1]
}

func (p *parseState) deleteLast() {
	ob := &p.stack[len(p.stack)-1]
	ob.inner = ob.inner[:len(ob.inner)-1]
}

type Text struct {
	Position
	Inline []Inline
}

func (b *Text) PrintHTML(buf *bytes.Buffer) {
	for _, x := range b.Inline {
		x.PrintHTML(buf)
	}
}

func (b *Text) printMarkdown(buf *markOut, s mdState) {
	for _, x := range b.Inline {
		x.printMarkdown(buf)
	}
}

type rootBuilder struct{}

func (b *rootBuilder) build(p buildState) Block {
	return &Document{p.pos(), p.blocks(), p.(*parseState).links}
}

type Document struct {
	Position
	Blocks []Block
	Links  map[string]*Link
}

// A Parser is a Markdown parser.
// The exported fields in the struct can be filled in before calling
// [Parser.Parse] in order to customize the details of the parsing process.
// A Parser is safe for concurrent use by multiple goroutines.
type Parser struct {
	// HeadingIDs determines whether the parser accepts
	// the {#hdr} syntax for an HTML id="hdr" attribute on headings.
	// For example, if HeadingIDs is true then the Markdown
	//    ## Overview {#overview}
	// will render as the HTML
	//    <h2 id="overview">Overview</h2>
	HeadingIDs bool

	// Strikethrough determines whether the parser accepts
	// ~abc~ and ~~abc~~ as strikethrough syntax, producing
	// <del>abc</del> in HTML.
	Strikethrough bool

	// TaskListItems determines whether the parser accepts
	// “task list items” as defined in GitHub Flavored Markdown.
	// When a list item begins with the plain text [ ] or [x]
	// that turns into an unchecked or checked check box.
	TaskListItems bool

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
}

type parseState struct {
	*Parser

	root      *Document
	links     map[string]*Link
	lineno    int
	stack     []openBlock
	lineDepth int

	corner bool // noticed corner case to ignore in cross-implementation testing

	// inlines
	s       string
	emitted int // s[:emitted] has been emitted into list
	list    []Inline

	// for fixup at end
	lists []*List
	texts []textRaw

	backticks backtickParser
}

type textRaw struct {
	*Text
	raw string
}

func (p *parseState) newText(pos Position, text string) *Text {
	b := &Text{Position: pos}
	p.texts = append(p.texts, textRaw{b, text})
	return b
}

func (p *parseState) blocks() []Block {
	b := &p.stack[len(p.stack)-1]
	return b.inner
}

func (p *parseState) pos() Position {
	b := &p.stack[len(p.stack)-1]
	return b.pos
}

func (p *Parser) Parse(text string) *Document {
	d, _ := p.parse(text)
	return d
}

func (p *Parser) parse(text string) (d *Document, corner bool) {
	var ps parseState
	ps.Parser = p
	if strings.Contains(text, "\x00") {
		text = strings.ReplaceAll(text, "\x00", "\uFFFD")
		ps.corner = true // goldmark does not replace NUL
	}

	ps.lineDepth = -1
	ps.addBlock(&rootBuilder{})
	for text != "" {
		var ln string
		i := strings.Index(text, "\n")
		j := strings.Index(text, "\r")
		var nl byte
		switch {
		case j >= 0 && (i < 0 || j < i): // have \r, maybe \r\n
			ln = text[:j]
			if i == j+1 {
				text = text[j+2:]
				nl = '\r' + '\n'
			} else {
				text = text[j+1:]
				nl = '\r'
			}
		case i >= 0:
			ln, text = text[:i], text[i+1:]
			nl = '\n'
		default:
			ln, text = text, ""
		}
		ps.lineno++
		ps.addLine(line{text: ln, nl: nl})
	}
	ps.trimStack(0)

	for _, t := range ps.texts {
		t.Inline = ps.inline(t.raw)
	}

	if p.TaskListItems {
		for _, list := range ps.lists {
			ps.taskList(list)
		}
	}

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

func (p *parseState) curB() blockBuilder {
	if p.lineDepth < len(p.stack) {
		return p.stack[p.lineDepth].builder
	}
	return nil
}

func (p *parseState) nextB() blockBuilder {
	if p.lineDepth+1 < len(p.stack) {
		return p.stack[p.lineDepth+1].builder
	}
	return nil
}
func (p *parseState) trimStack(depth int) {
	if len(p.stack) < depth {
		// unreachable
		panic("trimStack")
	}
	for len(p.stack) > depth {
		p.closeBlock()
	}
}

func (p *parseState) addBlock(c blockBuilder) {
	p.trimStack(p.lineDepth + 1)
	p.stack = append(p.stack, openBlock{})
	ob := &p.stack[len(p.stack)-1]
	ob.builder = c
	ob.pos.StartLine = p.lineno
	ob.pos.EndLine = p.lineno
}

func (p *parseState) doneBlock(b Block) {
	p.trimStack(p.lineDepth + 1)
	ob := &p.stack[len(p.stack)-1]
	ob.inner = append(ob.inner, b)
}

func (p *parseState) para() *paraBuilder {
	if b, ok := p.stack[len(p.stack)-1].builder.(*paraBuilder); ok {
		return b
	}
	return nil
}

func (p *parseState) closeBlock() Block {
	b := &p.stack[len(p.stack)-1]
	if b.builder == nil {
		println("closeBlock", len(p.stack)-1)
	}
	blk := b.builder.build(p)
	if list, ok := blk.(*List); ok {
		p.corner = p.corner || listCorner(list)
		if p.TaskListItems {
			p.lists = append(p.lists, list)
		}
	}
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

func (p *parseState) link(label string) *Link {
	return p.links[label]
}

func (p *parseState) defineLink(label string, link *Link) {
	if p.links == nil {
		p.links = make(map[string]*Link)
	}
	p.links[label] = link
}

type line struct {
	spaces int
	i      int
	tab    int
	text   string
	nl     byte // newline character ending this line: \r or \n or zero for EOF
}

func (p *parseState) addLine(s line) {
	// Process continued prefixes.
	p.lineDepth = 0
	for ; p.lineDepth+1 < len(p.stack); p.lineDepth++ {
		old := s
		var ok bool
		s, ok = p.stack[p.lineDepth+1].builder.extend(p, s)
		if !old.isBlank() && (ok || s != old) {
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
	for _, fn := range news {
		if l, ok := fn(p, s); ok {
			s = l
			if s.isBlank() {
				return
			}
			p.lineDepth++
			goto Prefixes
		}
	}

	newPara(p, s)
}

func (c *rootBuilder) extend(p *parseState, s line) (line, bool) {
	// unreachable
	panic("root extend")
}

var news = []func(*parseState, line) (line, bool){
	newQuote,
	newATXHeading,
	newSetextHeading,
	newHR,
	newListItem,
	newHTML,
	newFence,
	newPre,
}

func (s *line) peek() byte {
	if s.spaces > 0 {
		return ' '
	}
	if s.i >= len(s.text) {
		return 0
	}
	return s.text[s.i]
}

func (s *line) skipSpace() {
	s.spaces = 0
	for s.i < len(s.text) && (s.text[s.i] == ' ' || s.text[s.i] == '\t') {
		s.i++
	}
}

func (s *line) trimSpace(min, max int, eolOK bool) bool {
	t := *s
	for n := 0; n < max; n++ {
		if t.spaces > 0 {
			t.spaces--
			continue
		}
		if t.i >= len(t.text) && eolOK {
			continue
		}
		if t.i < len(t.text) {
			switch t.text[t.i] {
			case '\t':
				t.spaces = 4 - (t.i-t.tab)&3 - 1
				t.i++
				t.tab = t.i
				continue
			case ' ':
				t.i++
				continue
			}
		}
		if n >= min {
			break
		}
		return false
	}
	*s = t
	return true
}

func (s *line) trim(c byte) bool {
	if s.spaces > 0 {
		if c == ' ' {
			s.spaces--
			return true
		}
		return false
	}
	if s.i < len(s.text) && s.text[s.i] == c {
		s.i++
		return true
	}
	return false
}

func (s *line) string() string {
	switch s.spaces {
	case 0:
		return s.text[s.i:]
	case 1:
		return " " + s.text[s.i:]
	case 2:
		return "  " + s.text[s.i:]
	case 3:
		return "   " + s.text[s.i:]
	}
	// unreachable
	panic("bad spaces")
}

func trimLeftSpaceTab(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}

func trimRightSpaceTab(s string) string {
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[:j]
}

func trimSpaceTab(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	s = s[i:]
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[:j]
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	s = s[i:]
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[:j]
}

func trimSpaceTabNewline(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	s = s[i:]
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n') {
		j--
	}
	return s[:j]
}

func (s *line) isBlank() bool {
	return trimLeftSpaceTab(s.text[s.i:]) == ""
}

func (s *line) eof() bool {
	return s.i >= len(s.text)
}

func (s *line) trimSpaceString() string {
	return trimLeftSpaceTab(s.text[s.i:])
}

func (s *line) trimString() string {
	return trimSpaceTab(s.text[s.i:])
}

func ToHTML(b Block) string {
	var buf bytes.Buffer
	b.PrintHTML(&buf)
	return buf.String()
}

func Format(b Block) string {
	var buf markOut
	b.printMarkdown(&buf, mdState{})
	return buf.String()
}

func (b *Document) PrintHTML(buf *bytes.Buffer) {
	for _, c := range b.Blocks {
		c.PrintHTML(buf)
	}
}

func (b *Document) printMarkdown(buf *markOut, s mdState) {
	printMarkdownBlocks(b.Blocks, buf, s)

	// Terminate with a single newline.
	text := buf.Bytes()
	w := len(text)
	for w > 0 && text[w-1] == '\n' {
		w--
	}
	buf.Truncate(w)
	if w > 0 {
		buf.NL()
	}

	// Add link reference definitions.
	if len(b.Links) > 0 {
		if buf.Len() > 0 {
			buf.NL()
		}
		printLinks(buf, b.Links)
	}
}

func printMarkdownBlocks(bs []Block, buf *markOut, s mdState) {
	for bn, b := range bs {
		if bn > 0 {
			buf.NL() // end block
			if buf.loose > 0 {
				buf.NL()
			}
		}
		b.printMarkdown(buf, s)
	}
}
