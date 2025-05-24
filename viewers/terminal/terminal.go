package terminal

import _ "embed"

//go:embed compose.yaml
var compose string

type Terminal struct {
}

func NewTerminal() *Terminal {
	return &Terminal{}
}

func (t *Terminal) GetCompose() string {
	return compose
}

func (t *Terminal) GetExecArgs() []string {
	return []string{"app", "bash"}
}
