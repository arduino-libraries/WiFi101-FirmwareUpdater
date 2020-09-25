package utils

import (
	"log"
	"time"

	"go.bug.st/serial"
)

// http://www.ni.com/product-documentation/54548/en/
var baudRates = []int{
	// Standard baud rates supported by most serial ports
	115200,
	57600,
	56000,
	38400,
}

func OpenSerial(portName string) (serial.Port, error) {
	var port serial.Port
	var err error
	for _, baudRate := range baudRates {
		mode := &serial.Mode{
			BaudRate: baudRate,
		}
		port, err := serial.Open(portName, mode)
		if err == nil {
			log.Printf("Open the serial port with baud rate %d", baudRate)
			return port, nil
		}
		if err := port.SetReadTimeout(5 * time.Second); err != nil {
			log.Fatalf("Could not set timeout on serial port: %s", err)
			return nil, err
		}
	}
	return port, err

}