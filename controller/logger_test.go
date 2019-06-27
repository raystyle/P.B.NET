package controller

import (
	"testing"

	"project/internal/logger"
)

func Test_ctrl_logger(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	ctrl.Printf(logger.DEBUG, "test src", "test format %s", "test log")
	ctrl.Print(logger.DEBUG, "test src", "test print", "test log")
	ctrl.Println(logger.DEBUG, "test src", "test println", "test log")
}
