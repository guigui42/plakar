package agent

import (
	"github.com/PlakarKorp/plakar/scheduler"
	"github.com/PlakarKorp/plakar/scheduler/configparser"
)

func ParseConfigFile(path string) (*scheduler.Configuration, error) {
	return configparser.ParseConfigFile(path)
}
