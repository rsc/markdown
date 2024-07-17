// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strconv"
	"strings"
)

type Footnote struct {
	Position
	Label  string
	Blocks []Block
}

type FootnoteLink struct {
	Label    string
	Footnote *Footnote
}

type printedNote struct {
	num  string
	note *Footnote
	refs []string
}

func (*FootnoteLink) Inline() {}

func (x *Footnote) printed(p *printer) *printedNote {
	if p.footnotes == nil {
		p.footnotes = make(map[*Footnote]*printedNote)
	}
	pr, ok := p.footnotes[x]
	if !ok {
		pr = &printedNote{
			num:  strconv.Itoa(len(p.footnotes) + 1),
			note: x,
		}
		p.footnotes[x] = pr
		p.footnotelist = append(p.footnotelist, pr)
	}
	ref := pr.num
	if len(pr.refs) > 0 {
		ref += "-" + strconv.Itoa(len(pr.refs)+1)
	}
	pr.refs = append(pr.refs, ref)
	return pr
}

func (x *FootnoteLink) printHTML(p *printer) {
	note := x.Footnote
	if note == nil {
		return
	}
	pr := note.printed(p)
	ref := pr.refs[len(pr.refs)-1]
	p.html(`<sup class="fn"><a id="fnref-`, ref, `" href="#fn-`, pr.num, `">`, pr.num, `</a></sup>`)
}

func (x *FootnoteLink) printMarkdown(p *printer) {
	note := x.Footnote
	if note == nil {
		return
	}
	note.printed(p) // add to list for printFootnoteMarkdown
	p.text(`[^`, x.Label, `]`)
}

func (x *FootnoteLink) printText(p *printer) {
	p.text(`[^`, x.Label, `]`)
}

func printFootnoteHTML(p *printer) {
	if len(p.footnotelist) == 0 {
		return
	}

	p.html(`<div class="footnotes">Footnotes</div>`, "\n")
	p.html("<ol>\n")
	for num, note := range p.footnotelist {
		num++
		str := strconv.Itoa(num)
		p.html(`<li id="fn-`, str, `">`, "\n")
		for _, b := range note.note.Blocks {
			b.printHTML(p)
		}
		if !p.eraseCloseP() {
			p.html("<p>\n")
		}
		for _, ref := range note.refs {
			p.html("\n", `<a class="fnref" href="#fnref-`, ref, `">↩</a>`)
		}
		p.html("</p>\n")
		p.html("</li>\n")
	}
	p.html("</ol>\n")
}

func (x *Footnote) printMarkdown(p *printer) {
	p.md(`[^`, x.Label, `]: `)
	defer p.pop(p.push("  "))
	printMarkdownBlocks(x.Blocks, p)
}

func printFootnoteMarkdown(p *printer) {
	if len(p.footnotelist) == 0 {
		return
	}

	p.maybeNL()
	for _, note := range p.footnotelist {
		p.nl()
		note.note.printMarkdown(p)
	}
}

func parseFootnoteRef(p *parser, s string, start int) (x Inline, end int, ok bool) {
	if !p.Footnote || start+1 >= len(s) || s[start+1] != '^' {
		return
	}
	end = strings.Index(s[start:], "]")
	if end < 0 {
		return
	}
	end += start + 1
	label := s[start+2 : end-1]
	note, ok := p.footnotes[normalizeLabel(label)]
	if !ok {
		return
	}
	return &FootnoteLink{label, note}, end, true
}

func startFootnote(p *parser, s line) (line, bool) {
	t := s
	t.trimSpace(0, 3, false)
	if !t.trim('[') || !t.trim('^') {
		return s, false
	}
	label := t.string()
	i := strings.Index(label, "]")
	if i < 0 || i+1 >= len(label) && label[i+1] != ':' {
		return s, false
	}
	label = label[:i]
	for j := 0; j < i; j++ {
		c := label[j]
		if c == ' ' || c == '\r' || c == '\n' || c == 0x00 || c == '\t' {
			return s, false
		}
	}
	t.skip(i + 2)

	if _, ok := p.footnotes[normalizeLabel(label)]; ok {
		// Already have a footnote with this label.
		// cmark-gfm ignores all future references,
		// dropping them from the document,
		// but it seems more helpful to not treat it
		// as a footnote.
		p.corner = true
		return s, false
	}

	fb := &footnoteBuilder{label}
	p.addBlock(fb)
	return t, true
}

type footnoteBuilder struct {
	label string
}

func (b *footnoteBuilder) extend(p *parser, s line) (line, bool) {
	if !s.trimSpace(4, 4, true) {
		return s, false
	}
	return s, true
}

func (b *footnoteBuilder) build(p *parser) Block {
	if p.footnotes == nil {
		p.footnotes = make(map[string]*Footnote)
	}
	p.footnotes[normalizeLabel(b.label)] = &Footnote{p.pos(), b.label, p.blocks()}
	return &Empty{}
}
