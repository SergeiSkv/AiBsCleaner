" AiBsCleaner.vim - AI Bullshit Cleaner for Vim/Neovim
" Author: AiBsCleaner Team
" License: MIT

if exists('g:loaded_aibscleaner')
    finish
endif
let g:loaded_aibscleaner = 1

" Configuration
let g:aibscleaner_auto_run = get(g:, 'aibscleaner_auto_run', 1)
let g:aibscleaner_min_severity = get(g:, 'aibscleaner_min_severity', 'MEDIUM')
let g:aibscleaner_format = get(g:, 'aibscleaner_format', 'json')

" Commands
command! AiBsClean call aibscleaner#Run()
command! AiBsCleanFix call aibscleaner#Fix()
command! AiBsCleanToggle call aibscleaner#Toggle()

" Keybindings
nnoremap <silent> <leader>ab :AiBsClean<CR>
nnoremap <silent> <leader>af :AiBsCleanFix<CR>

" Auto-run on save for Go files
augroup AiBsCleaner
    autocmd!
    if g:aibscleaner_auto_run
        autocmd BufWritePost *.go silent! call aibscleaner#RunBackground()
    endif
augroup END

" Main function
function! aibscleaner#Run() abort
    let l:filename = expand('%:p')
    if &filetype != 'go'
        echo "AiBsCleaner: Only works with Go files"
        return
    endif

    echo "Running AiBsCleaner..."
    let l:cmd = 'aibscleaner -path ' . shellescape(l:filename) . ' -format json'
    let l:output = system(l:cmd)
    
    if v:shell_error
        echo "AiBsCleaner: Error running analysis"
        return
    endif

    call aibscleaner#ProcessResults(l:output)
endfunction

" Process and display results
function! aibscleaner#ProcessResults(output) abort
    try
        let l:results = json_decode(a:output)
    catch
        echo "AiBsCleaner: Failed to parse results"
        return
    endtry

    " Clear existing signs
    sign unplace *

    " Create quickfix list
    let l:qflist = []
    
    for issue in get(l:results, 'issues', [])
        " Add to quickfix
        call add(l:qflist, {
            \ 'filename': issue.file,
            \ 'lnum': issue.line,
            \ 'col': issue.column,
            \ 'text': issue.type . ': ' . issue.message,
            \ 'type': issue.severity == 'HIGH' ? 'E' : issue.severity == 'MEDIUM' ? 'W' : 'I'
        \ })

        " Add sign
        let l:sign_name = 'AiBsCleaner' . issue.severity
        execute 'sign place ' . issue.line . ' line=' . issue.line . 
            \ ' name=' . l:sign_name . ' file=' . expand('%:p')
    endfor

    " Set quickfix list
    call setqflist(l:qflist)
    
    " Show results count
    let l:count = len(l:qflist)
    if l:count > 0
        echo printf("AiBsCleaner: Found %d issue%s", l:count, l:count == 1 ? '' : 's')
        copen
    else
        echo "AiBsCleaner: No issues found!"
        cclose
    endif
endfunction

" Fix issues automatically
function! aibscleaner#Fix() abort
    let l:filename = expand('%:p')
    echo "Running AiBsCleaner auto-fix..."
    let l:cmd = 'aibscleaner -path ' . shellescape(l:filename) . ' --fix'
    let l:output = system(l:cmd)
    
    if v:shell_error
        echo "AiBsCleaner: Error fixing issues"
    else
        edit!
        echo "AiBsCleaner: Fixed issues"
    endif
endfunction

" Background run for auto-save
function! aibscleaner#RunBackground() abort
    if !exists('*job_start')  " Vim 8+ or Neovim
        return
    endif
    
    let l:filename = expand('%:p')
    let l:cmd = ['aibscleaner', '-path', l:filename, '-format', 'json']
    
    if has('nvim')
        call jobstart(l:cmd, {
            \ 'on_stdout': function('s:OnOutput'),
            \ 'on_exit': function('s:OnExit')
        \ })
    else
        call job_start(l:cmd, {
            \ 'out_cb': function('s:OnOutput'),
            \ 'exit_cb': function('s:OnExit')
        \ })
    endif
endfunction

" Define signs
sign define AiBsCleanerHIGH text=🔴 texthl=Error
sign define AiBsCleanerMEDIUM text=🟡 texthl=Warning
sign define AiBsCleanerLOW text=🔵 texthl=Information

" Toggle auto-run
function! aibscleaner#Toggle() abort
    let g:aibscleaner_auto_run = !g:aibscleaner_auto_run
    echo "AiBsCleaner auto-run: " . (g:aibscleaner_auto_run ? "enabled" : "disabled")
endfunction