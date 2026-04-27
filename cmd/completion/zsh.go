package completion

import "io"

func zshCompletion(w io.Writer) error {
	script := `#!/usr/bin/env zsh

# Zsh completion for please
_please() {
    local -a opts
    opts=(
        '-n[Start a new session]'
        '--new[Start a new session]'
        '-1[Take a turn without updating session]'
        '--one-off[Take a turn without updating session]'
        '-h[Show help]'
        '--help[Show help]'
        '--completion[Generate completion script]'
    )
    _arguments -s $opts '*:message:_files'
}

_compdef _please please
`
	_, err := w.Write([]byte(script))
	return err
}
