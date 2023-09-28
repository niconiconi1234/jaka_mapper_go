package main

import (
	"github.com/kubeedge/mappers-go/mapper-sdk-go/pkg/service"
	"github.com/niconiconi1234/jaka_mapper_go/driver"
)

func main() {
	d := driver.JakaDriver{}
	service.Bootstrap("jaka-protocol", &d)
}
