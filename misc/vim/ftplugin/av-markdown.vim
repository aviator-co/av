if exists("b:did_ftplugin") | finish | endif
let b:did_ftplugin = 1
let s:keepcpo= &cpo
set cpo&vim

setlocal comments=b:%%
setlocal commentstring=%%\ %s

let b:undo_ftplugin = 'setlocal comments<'
      \ . '|setlocal commentstring<'
      \ . '|unlet! b:undo_ftplugin'

let &cpo = s:keepcpo
unlet s:keepcpo
