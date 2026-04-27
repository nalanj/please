package completion

import "io"

func fishCompletion(w io.Writer) error {
	script := `# Fish completion for please

complete -c please -s n -l new -d "Start a new session"
complete -c please -s 1 -l one-off -d "Take a turn without updating session"
complete -c please -s h -l help -d "Show help"
complete -c please -l completion -d "Generate completion script"
`
	_, err := w.Write([]byte(script))
	return err
}
