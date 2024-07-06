// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

// A Quote is a [Block] representing a [block quote].
//
// [block quote]: https://spec.commonmark.org/0.31.2/#block-quotes
type Quote struct {
	Position
	Blocks []Block // content of quote
}

func (*Quote) Block() {}

func (b *Quote) printHTML(p *printer) {
	p.html("<blockquote>\n")
	for _, c := range b.Blocks {
		c.printHTML(p)
	}
	p.html("</blockquote>\n")
}

func (b *Quote) printMarkdown(p *printer) {
	p.maybeQuoteNL('>')
	p.WriteString("> ")
	defer p.pop(p.push("> "))
	printMarkdownBlocks(b.Blocks, p)
}

// A quoteBuildier is a [blockBuilder] for a block quote.
type quoteBuilder struct{}

// startBlockQuote is a [starter] for a [Quote].
func startBlockQuote(p *parser, s line) (line, bool) {
	line, ok := trimQuote(s)
	if !ok {
		return s, false
	}
	p.addBlock(new(quoteBuilder))
	return line, true
}

func trimQuote(s line) (line, bool) {
	t := s
	t.trimSpace(0, 3, false)
	if !t.trim('>') {
		return s, false
	}
	t.trimSpace(0, 1, true)
	return t, true
}

func (b *quoteBuilder) extend(p *parser, s line) (line, bool) {
	return trimQuote(s)
}

func (b *quoteBuilder) build(p *parser) Block {
	return &Quote{p.pos(), p.blocks()}
}
