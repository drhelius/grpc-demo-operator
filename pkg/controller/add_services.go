package controller

import (
	"github.com/drhelius/grpc-demo-operator/pkg/controller/services"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, services.Add)
}
