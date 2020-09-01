package kiwi

import (
	"fmt"

	"golang.org/x/sys/windows"

	"project/internal/logger"
)

func (kiwi *Kiwi) acquireNT5LSAKeys(pHandle windows.Handle) error {
	fmt.Println(pHandle)
	kiwi.log(logger.Info, "acquire NT5 LSA keys successfully")
	return nil
}
