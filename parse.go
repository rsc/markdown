// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"reflect"
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

type Block interface {
	Pos() Position
	PrintHTML(buf *bytes.Buffer)
}

type Position struct {
	StartLine int
	EndLine   int
}

func (p Position) Pos() Position {
	return p
}

type BuildState interface {
	Blocks() []Block
	Pos() Position
	Last() Block
	DeleteLast()

	Link(label string) *Link
	DefineLink(label string, link *Link)
	NewText(pos Position, text string) Block
}

type BlockBuilder interface {
	Extend(p *parser, line Line) (Line, bool)
	Build(BuildState) Block
}

type openBlock struct {
	builder BlockBuilder
	inner   []Block
	pos     Position
}

type itemBuilder struct {
	list        *listBuilder
	width       int
	haveContent bool
}

func (p *parser) Last() Block {
	ob := &p.stack[len(p.stack)-1]
	return ob.inner[len(ob.inner)-1]
}

func (p *parser) DeleteLast() {
	ob := &p.stack[len(p.stack)-1]
	ob.inner = ob.inner[:len(ob.inner)-1]
}

type Text struct {
	Position
	Inline []Inline
	raw    string
}

func (b *Text) PrintHTML(buf *bytes.Buffer) {
	for _, x := range b.Inline {
		x.PrintHTML(buf)
	}
}

type rootBuilder struct{}

func (b *rootBuilder) Build(p BuildState) Block {
	return &Document{p.Pos(), p.Blocks(), p.(*parser).links}
}

type Document struct {
	Position
	Blocks []Block
	Links  map[string]*Link
}

type parser struct {
	root      Block
	links     map[string]*Link
	lineno    int
	stack     []openBlock
	lineDepth int

	// inlines
	s       string
	emitted int // s[:emitted] has been emitted into list
	list    []Inline

	texts []*Text
}

func (p *parser) NewText(pos Position, text string) Block {
	b := &Text{Position: pos, raw: text}
	p.texts = append(p.texts, b)
	return b
}

func (p *parser) Blocks() []Block {
	b := &p.stack[len(p.stack)-1]
	return b.inner
}

func (p *parser) Pos() Position {
	b := &p.stack[len(p.stack)-1]
	return b.pos
}

func parse(text string) Block {
	text = strings.ReplaceAll(text, "\x00", "\uFFFD")
	var p parser
	p.lineDepth = -1
	p.addBlock(&rootBuilder{})
	for text != "" {
		var line string
		i := strings.Index(text, "\n")
		j := strings.Index(text, "\r")
		switch {
		case j >= 0 && (i < 0 || j < i): // have \r, maybe \r\n
			line = text[:j]
			if i == j+1 {
				text = text[j+2:]
			} else {
				text = text[j+1:]
			}
		case i >= 0:
			line, text = text[:i], text[i+1:]
		default:
			line, text = text, ""
		}
		p.lineno++
		p.addLine(Line{text: line})
	}
	p.trimStack(0)

	for _, t := range p.texts {
		t.Inline = p.inline(t.raw)
	}

	return p.root
}

func (p *parser) curB() BlockBuilder {
	if p.lineDepth < len(p.stack) {
		return p.stack[p.lineDepth].builder
	}
	return nil
}

func (p *parser) nextB() BlockBuilder {
	if p.lineDepth+1 < len(p.stack) {
		return p.stack[p.lineDepth+1].builder
	}
	return nil
}
func (p *parser) trimStack(depth int) {
	if len(p.stack) < depth {
		panic("trimStack")
	}
	for len(p.stack) > depth {
		p.closeBlock()
	}
}

func (p *parser) addBlock(c BlockBuilder) {
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
	blk := b.builder.Build(p)
	p.stack = p.stack[:len(p.stack)-1]
	if len(p.stack) > 0 {
		b := &p.stack[len(p.stack)-1]
		b.inner = append(b.inner, blk)
		// _ = b
	} else {
		p.root = blk
	}
	return blk
}

func (p *parser) Link(label string) *Link {
	return p.links[label]
}

func (p *parser) DefineLink(label string, link *Link) {
	if p.links == nil {
		p.links = make(map[string]*Link)
	}
	p.links[label] = link
}

type Line struct {
	spaces int
	i      int
	tab    int
	text   string
}

func (p *parser) addLine(line Line) {
	// Process continued prefixes.
	p.lineDepth = 0
	for ; p.lineDepth+1 < len(p.stack); p.lineDepth++ {
		old := line
		var ok bool
		line, ok = p.stack[p.lineDepth+1].builder.Extend(p, line)
		if !old.isBlank() && (ok || line != old) {
			p.stack[p.lineDepth+1].pos.EndLine = p.lineno
		}
		if !ok {
			break
		}
	}

	if line.isBlank() {
		p.trimStack(p.lineDepth + 1)
		return
	}

	// Process new prefixes, if any.
Prefixes:
	// Start new block inside p.stack[depth].
	for _, fn := range news {
		if l, ok := fn(p, line); ok {
			line = l
			if line.isBlank() {
				return
			}
			p.lineDepth++
			goto Prefixes
		}
	}

	newPara(p, line)
}

func (c *rootBuilder) Extend(p *parser, line Line) (Line, bool) {
	panic("root extend")
}

var news = []func(*parser, Line) (Line, bool){
	newQuote,
	newATXHeading,
	newSetextHeading,
	newHR,
	newListItem,
	newHTML,
	newFence,
	newPre,
}

func (s *Line) peek() byte {
	if s.spaces > 0 {
		return ' '
	}
	if s.i >= len(s.text) {
		return 0
	}
	return s.text[s.i]
}

func (s *Line) skipSpace() {
	s.spaces = 0
	for s.i < len(s.text) && (s.text[s.i] == ' ' || s.text[s.i] == '\t') {
		s.i++
	}
}

func (s *Line) trimSpace(min, max int, eolOK bool) bool {
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

func (s *Line) trim(c byte) bool {
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

func (s *Line) string() string {
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
	panic("bad spaces")
}

func (s *Line) isBlank() bool {
	return strings.Trim(s.text[s.i:], " \t") == ""
}

func (s *Line) eof() bool {
	return s.i >= len(s.text)
}

func (s *Line) trimSpaceString() string {
	return strings.TrimLeft(s.text[s.i:], " \t")
}

func (s *Line) trimString() string {
	return strings.Trim(s.text[s.i:], " \t")
}

func toHTML(b Block) string {
	var buf bytes.Buffer
	b.PrintHTML(&buf)
	return buf.String()
}

func (b *Document) PrintHTML(buf *bytes.Buffer) {
	for _, c := range b.Blocks {
		c.PrintHTML(buf)
	}
}

var blocksType = reflect.TypeOf(new([]Block)).Elem()

func printb(buf *bytes.Buffer, b Block, prefix string) {
	fmt.Fprintf(buf, "(%T", b)
	v := reflect.ValueOf(b)
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		fmt.Fprintf(buf, " %v", b)
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		if !tf.IsExported() {
			continue
		}
		if tf.Type != blocksType {
			fmt.Fprintf(buf, " %s:%v", tf.Name, v.Field(i))
		}
	}

	prefix += "\t"
	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		if !tf.IsExported() {
			continue
		}
		if tf.Type == blocksType {
			vf := v.Field(i)
			for i := 0; i < vf.Len(); i++ {
				fmt.Fprintf(buf, "\n%s", prefix)
				printb(buf, vf.Index(i).Interface().(Block), prefix)
			}
		}
	}
	fmt.Fprintf(buf, ")")
}

func dump(b Block) string {
	var buf bytes.Buffer
	printb(&buf, b, "")
	return buf.String()
}
