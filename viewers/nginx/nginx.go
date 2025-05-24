package nginx

import _ "embed"

//go:embed compose.yaml
var compose string

type Nginx struct {
}

func NewNginx() *Nginx {
	return &Nginx{}
}

func (t *Nginx) GetCompose() string {
	return compose
}
