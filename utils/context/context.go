package context

import (
	"fmt"
)

type addressFlags []string

func (af *addressFlags) String() string {
	return fmt.Sprint(*af)
}

func (af *addressFlags) Set(value string) error {
	*af = append(*af, value)
	return nil
}

type Context struct {
	PortName         string
	RootCertDir      string
	Addresses        addressFlags
	FirmwareFile     string
	FWUploaderBinary string
	ReadAll          bool
	BinaryToRestore  string
	ProgrammerPath   string
	Model            string
	Compatible       string
}

type Programmer interface {
	Flash(filename string) error
}
