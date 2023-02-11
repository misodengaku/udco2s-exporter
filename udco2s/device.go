package udco2s

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

type UDCO2S struct {
	port        serial.Port
	ID          string
	Version     string
	FRC         int
	Timestamp   time.Time
	CO2         int
	Humidity    float64
	Temperature float64
}

func (u *UDCO2S) Init(ctx context.Context, devicePath string) error {
	port, err := serial.Open(devicePath, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		return err
	}
	u.port = port

	go func() {
		var lineBuffer string

		for {
			select {
			case <-ctx.Done():
				return
			default:
				buffer := make([]byte, 128)
				n, err := u.port.Read(buffer)
				if err != nil {
					panic(err)
				}
				if n == 0 {
					continue
				}
				lineBuffer += strings.Trim(string(buffer), "\x00")

				lines := strings.Split(lineBuffer, "\r\n")
				if len(lines) == 1 {
					continue
				}
				for _, line := range lines[:len(lines)-1] {
					line = strings.TrimSuffix(line, "\r\n")
					line = strings.TrimPrefix(line, "OK ")
					u.parseLine(line)

				}
				lineBuffer = lines[len(lines)-1]
			}
		}
	}()
	return nil
}

func (u *UDCO2S) parseLine(line string) error {
	var err error
	elems := strings.Split(line, ",")
	for _, elem := range elems {
		kv := strings.Split(elem, "=")
		switch kv[0] {
		case "CO2":
			u.CO2, err = strconv.Atoi(kv[1])
			if err != nil {
				return err
			}
			u.Timestamp = time.Now()
		case "HUM":
			u.Humidity, err = strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return err
			}
			u.Timestamp = time.Now()
		case "TMP":
			u.Temperature, err = strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return err
			}
			u.Timestamp = time.Now()
		case "ID":
			u.ID = kv[1]
		case "VER":
			u.Version = kv[1]
		}
	}
	return nil
}

func (u *UDCO2S) readResult() (result string, success bool) {
	buffer := make([]byte, 1024)
	result = strings.TrimRight(string(buffer), " \r\n")
	success = result != "NG"
	if success {
		result = strings.TrimPrefix(result, "OK ")
	}
	return
}

func (u *UDCO2S) GetDeviceID() string {
	u.port.Write([]byte("ID?\r\n"))
	result, ok := u.readResult()
	if !ok {
		panic(result)
	}
	return result
}

func (u *UDCO2S) GetFirmwareVersion() string {
	u.port.ResetInputBuffer()
	u.port.Write([]byte("VER?\r\n"))
	result, ok := u.readResult()
	if !ok {
		panic(result)
	}
	return result
}

func (u *UDCO2S) StartMeasurement() string {
	u.port.ResetInputBuffer()
	u.port.Write([]byte("STA\r\n"))
	result, ok := u.readResult()
	if !ok {
		panic(result)
	}
	return result
}

func (u *UDCO2S) StopMeasurement() {
	u.port.Write([]byte("STP\r\n"))
}

func (u *UDCO2S) GetFRCValue() {
	u.port.Write([]byte("FRC?\r\n"))
}

func (u *UDCO2S) setFRCValue(frcValue int) error {
	if frcValue < 400 || frcValue > 2000 {
		return errors.New("frcValue is out of range")
	}
	cmd := fmt.Sprintf("FRC=%d\r\n", frcValue)
	u.port.Write([]byte(cmd))
	return nil
}
