// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"
)

// A CodeBlock is a [Block] representing an [indented code block]
// or [fenced code block],
// usually displayed in <pre><code> tags.
//
// When printing a CodeBlock as Markdown, the Fence field is used as
// a starting hint but is made longer as needed if the suggested fence text
// appears in Text.
//
// [indented code block]: https://spec.commonmark.org/0.31.2/#indented-code-blocks
// [fenced code block]: https://spec.commonmark.org/0.31.2/#fenced-code-blocks
type CodeBlock struct {
	Position
	Fence string   // fence to use
	Info  string   // info following open fence
	Text  []string // lines of code block
}

func (*CodeBlock) Block() {}

func (b *CodeBlock) printHTML(p *printer) {
	p.html("<pre><code")
	if b.Info != "" {
		// https://spec.commonmark.org/0.31.2/#info-string
		// “The first word of the info string is typically used to
		// specify the language of the code sample...”
		// No definition of what “first word” means though.
		// The Dingus splits on isUnicodeSpace, but Goldmark only uses space.
		lang := b.Info
		for i, c := range lang {
			if isUnicodeSpace(c) {
				lang = lang[:i]
				break
			}
		}
		p.html(` class="language-`)
		p.text(lang)
		p.html(`"`)
	}
	p.WriteString(">")
	for _, s := range b.Text {
		p.text(s, "\n")
	}
	p.html("</code></pre>\n")
}

func (b *CodeBlock) printMarkdown(p *printer) {
	if b.Fence == "" {
		p.maybeNL()
		for i, line := range b.Text {
			if i > 0 {
				p.nl()
			}
			p.md("    ")
			p.md(line)
			p.noTrim()
		}
	} else {
		// TODO compute correct fence
		if p.tight == 0 {
			p.maybeNL()
		}
		p.md(b.Fence)
		p.md(b.Info)
		for _, line := range b.Text {
			p.nl()
			p.md(line)
			p.noTrim()
		}
		p.nl()
		p.md(b.Fence)
	}
}

// startIndentedCodeBlock is a [starter] for an indented [CodeBlock].
// See https://spec.commonmark.org/0.31.2/#indented-code-blocks.
func startIndentedCodeBlock(p *parser, s line) (line, bool) {
	// Line must start with 4 spaces and then not be blank.
	peek := s
	if p.para() != nil || !peek.trimSpace(4, 4, false) || peek.isBlank() {
		return s, false
	}

	b := &indentBuilder{}
	p.addBlock(b)
	if peek.nl != '\n' {
		p.corner = true // goldmark does not normalize to \n
	}
	b.text = append(b.text, peek.string())
	return line{}, true
}

// startFencedCodeBlock is a [starter] for a fenced [CodeBlock].
// See https://spec.commonmark.org/0.31.2/#fenced-code-blocks.
func startFencedCodeBlock(p *parser, s line) (line, bool) {
	// Line must start with fence.
	indent, fence, info, ok := trimFence(&s)
	if !ok {
		return s, false
	}

	// Note presence of corner cases, for testing.
	if fence[0] == '~' && info != "" {
		// goldmark does not handle info after ~~~
		p.corner = true
	} else if info != "" && !isLetter(info[0]) {
		// goldmark does not allow numbered info.
		// goldmark does not treat a tab as introducing a new word.
		p.corner = true
	}
	for _, c := range info {
		if isUnicodeSpace(c) {
			if c != ' ' {
				// goldmark only breaks on space
				p.corner = true
			}
			break
		}
	}

	p.addBlock(&fenceBuilder{indent, fence, info, nil})
	return line{}, true
}

// trimFence attempts to trim leading indentation (up to 3 spaces),
// a code fence, and an info string from s.
// If successful, it returns those values and ok=true, leaving s empty.
// If unsuccessful, it leaves s unmodified and returns ok=false.
func trimFence(s *line) (indent int, fence, info string, ok bool) {
	t := *s
	indent = 0
	for indent < 3 && t.trimSpace(1, 1, false) {
		indent++
	}
	c := t.peek()
	if c != '`' && c != '~' {
		return
	}

	f := t.string()
	n := 0
	for t.trim(c) {
		n++
	}
	if n < 3 {
		return
	}

	txt := mdUnescaper.Replace(t.trimString())
	if c == '`' && strings.Contains(txt, "`") {
		return
	}
	info = trimSpaceTab(txt)
	fence = f[:n]
	ok = true
	*s = line{}
	return
}

// An indentBuilder is a [blockBuilder] for an indented (unfenced) [CodeBlock].
type indentBuilder struct {
	indent string
	text   []string
}

func (c *indentBuilder) extend(p *parser, s line) (line, bool) {
	// Extension lines must start with 4 spaces or be blank.
	if !s.trimSpace(4, 4, true) {
		return s, false
	}
	c.text = append(c.text, s.string())
	if s.nl != '\n' {
		p.corner = true // goldmark does not normalize to \n
	}
	return line{}, true
}

func (b *indentBuilder) build(p *parser) Block {
	// Remove trailing blank lines, which are often used
	// just to separate the indented code block from what follows.
	for len(b.text) > 0 && b.text[len(b.text)-1] == "" {
		b.text = b.text[:len(b.text)-1]
	}
	return &CodeBlock{p.pos(), "", "", b.text}
}

// A fenceBuilder is a [blockBuilder] for a fenced [CodeBlock].
type fenceBuilder struct {
	indent int
	fence  string
	info   string
	text   []string
}

func (c *fenceBuilder) extend(p *parser, s line) (line, bool) {
	// Check for closing fence, which must be at least as long as opening fence, with no info.
	// The closing fence can be indented less than the opening one.
	peek := s
	if _, fence, info, ok := trimFence(&peek); ok && strings.HasPrefix(fence, c.fence) && info == "" {
		return line{}, false
	}

	// Otherwise trim the indentation from the fence line, if present.
	if !s.trimSpace(c.indent, c.indent, false) {
		p.corner = true // goldmark mishandles fenced blank lines with not enough spaces
		s.trimSpace(0, c.indent, false)
	}

	c.text = append(c.text, s.string())
	p.corner = p.corner || s.nl != '\n' // goldmark does not normalize to \n
	return line{}, true
}

func (c *fenceBuilder) build(p *parser) Block {
	return &CodeBlock{p.pos(), c.fence, c.info, c.text}
}
