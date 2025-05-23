package viewers

import _ "embed"

//go:embed terminal.yaml
var compose string

type Terminal struct {
}

func NewTerminal() *Terminal {
	return &Terminal{}
}

func (t *Terminal) GetCompose() string {
	return compose
}

func (t *Terminal) Attach() string {
	return "app"
}
