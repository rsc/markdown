Package markdown is a Commonmark-compliant Markdown parser and
HTML generator. It does not have many bells and whistles, but it does
expose the parsed syntax in an easy-to-use form.

Work in progress.

TODO:
 - documentation
 - make Format always print valid markdown,
   even when the tree was constructed manually and may
   not correspond to something Parse would return.
 - footnote support
 - possibly math support
 - would it be simpler to have a lexer generated from regexps?
