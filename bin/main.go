package main

import (
	"github.com/docker/machine/libmachine/drivers/plugin"
	"github.com/venusource/docker-machine-aliyun"
)

func main() {
	plugin.RegisterDriver(aliyun.NewDriver("", ""))
}
