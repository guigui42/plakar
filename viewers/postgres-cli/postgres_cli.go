package postgres_cli

import _ "embed"

//go:embed compose.yaml
var compose string

type PostgresCLI struct {
}

func NewPostgresCLI() *PostgresCLI {
	return &PostgresCLI{}
}

func (t *PostgresCLI) GetCompose() string {
	return compose
}

func (t *PostgresCLI) GetExecArgs() []string {
	return []string{"db", "psql", "-U", "postgres", "plakar"}
}
