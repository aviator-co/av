if exists("b:current_syntax")
  finish
endif

if !exists('main_syntax')
  let main_syntax = 'av-markdown'
endif

runtime! syntax/markdown.vim
unlet! b:current_syntax

syn match avMarkdownComment "^\s*%%.*$" contains=avMarkdownTodo,@Spell
syn keyword avMarkdownTodo FIXME NOTE NOTES TODO XXX contained

hi def link avMarkdownComment Comment

let b:current_syntax = "av-markdown"
if main_syntax ==# 'av-markdown'
  unlet main_syntax
endif
