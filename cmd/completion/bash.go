package completion

import "io"

func bashCompletion(w io.Writer) error {
	script := `#!/bin/bash

# Bash completion for please
_please_complete() {
    local cur prev words cword
    _init_completion || return

    # Option completion
    if [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "-n --new -1 --one-off -h --help --completion" -- "$cur"))
        return
    fi
}

complete -F _please_complete please
`
	_, err := w.Write([]byte(script))
	return err
}
