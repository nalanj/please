package completion

import "io"

func powershellCompletion(w io.Writer) error {
	script := `# PowerShell completion for please
Register-ArgumentCompleter -CommandName please -ParameterName completion -ScriptBlock {
    param($wordToComplete, $commandName, $cursorPosition)
    $completions = @('bash', 'zsh', 'fish', 'powershell')
    $completions | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}

Register-ArgumentCompleter -CommandName please -ScriptBlock {
    param($wordToComplete, $commandName, $commandAst, $cursorPosition)
    $options = @('-n', '--new', '-1', '--one-off', '-h', '--help', '--completion')
    $options | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`
	_, err := w.Write([]byte(script))
	return err
}
