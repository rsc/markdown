// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

type Document struct {
	Position
	Blocks []Block
	Links  map[string]*Link
}

func (*Document) Block() {}

func (b *Document) printHTML(p *printer) {
	for _, c := range b.Blocks {
		c.printHTML(p)
	}
}

func (b *Document) printMarkdown(p *printer) {
	printMarkdownBlocks(b.Blocks, p)

	// Terminate with a single newline.
	text := p.buf.Bytes()
	w := len(text)
	for w > 0 && text[w-1] == '\n' {
		w--
	}
	p.buf.Truncate(w)
	if w > 0 {
		p.nl()
	}

	// Add link reference definitions.
	if len(b.Links) > 0 {
		if p.buf.Len() > 0 {
			p.nl()
		}
		printLinks(p, b.Links)
	}
}

func printMarkdownBlocks(bs []Block, p *printer) {
	for bn, b := range bs {
		if bn > 0 {
			p.nl() // end block
			if p.loose > 0 {
				p.nl()
			}
		}
		b.printMarkdown(p)
	}
}
