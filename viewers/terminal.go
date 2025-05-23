package viewers

import _ "embed"

//go:embed terminal.yaml
var terminalCompose string

type Terminal struct {
}

func NewTerminal() *Terminal {
	return &Terminal{}
}

func (t *Terminal) GetCompose() string {
	return terminalCompose
}

func (t *Terminal) GetExecArgs() []string {
	return []string{"app", "bash"}
}
