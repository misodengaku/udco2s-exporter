package udco2s

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

type UDCO2S struct {
	deviceID    string
	version     string
	frcValue    int
	timestamp   time.Time
	co2Value    int
	humidity    float64
	temperature float64

	mutex *sync.Mutex
	port  serial.Port
}

func (u *UDCO2S) Init(devicePath string) error {
	port, err := serial.Open(devicePath, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		return err
	}
	u.port = port
	u.mutex = &sync.Mutex{}

	return nil
}

func (u *UDCO2S) readLines(ctx context.Context, remainBuffer string) (line []string, remain string) {
	remain = remainBuffer
	for {
		select {
		case <-ctx.Done():
			return line, remain
		default:
			buffer := make([]byte, 128)
			n, err := u.port.Read(buffer)
			if err != nil {
				panic(err)
			}
			if n == 0 {
				continue
			}
			fragment := strings.Trim(string(buffer), "\x00")
			slog.Debug("received fragment:", "data", fragment)
			remain += fragment

			lines := strings.Split(remain, "\r\n")
			if len(lines) == 1 {
				continue
			}
			return lines[:len(lines)-1], lines[len(lines)-1]
		}
	}
}

func (u *UDCO2S) parseLine(line string) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	slog.Debug("parsing line:", "line", line)

	var err error
	elems := strings.Split(line, ",")
	for _, elem := range elems {
		kv := strings.Split(elem, "=")
		switch kv[0] {
		case "CO2":
			u.co2Value, err = strconv.Atoi(kv[1])
			if err != nil {
				return err
			}
			u.timestamp = time.Now()
		case "HUM":
			u.humidity, err = strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return err
			}
			u.timestamp = time.Now()
		case "TMP":
			u.temperature, err = strconv.ParseFloat(kv[1], 64)
			if err != nil {
				return err
			}
			u.timestamp = time.Now()
		case "ID":
			u.deviceID = kv[1]
		case "VER":
			u.version = kv[1]
		}
	}
	slog.Info("measurement value updated", "co2", u.co2Value, "humidity", u.humidity, "temperature", u.temperature)
	return nil
}

func (u *UDCO2S) readResult(ctx context.Context) (result, remain string, success bool) {
	var lines []string
	// var remain string
	success = false

	for range 10 {
		lines, remain = u.readLines(ctx, remain)
		for _, line := range lines {

			result = strings.TrimRight(line, " \r\n")
			if strings.HasPrefix(result, "OK ") {
				result = strings.TrimPrefix(result, "OK ")
				success = true
				return
			}
		}
	}
	return
}

func (u *UDCO2S) queryDeviceID(ctx context.Context) string {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.port.Write([]byte("ID?\r\n"))
	result, _, ok := u.readResult(ctx)
	if !ok {
		panic(result)
	}
	return result
}

func (u *UDCO2S) queryFirmwareVersion(ctx context.Context) string {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.port.ResetInputBuffer()
	u.port.Write([]byte("VER?\r\n"))
	result, _, ok := u.readResult(ctx)
	if !ok {
		panic(result)
	}
	return result
}

func (u *UDCO2S) StartMeasurement(ctx context.Context) (ok bool) {
	var remain string

	u.mutex.Lock()
	u.port.ResetInputBuffer()
	u.port.Write([]byte("STA\r\n"))
	_, remain, ok = u.readResult(ctx)
	if !ok {
		slog.Error("failed to start measurement")
		_ = u.port.Close()
		u.mutex.Unlock()
		return
	}

	u.mutex.Unlock()

	go func() {
		slog.Debug("starting measurement in background")
		var lines []string
		var lineBuffer string = remain

		for {
			select {
			case <-ctx.Done():
				return
			default:
				lines, lineBuffer = u.readLines(ctx, lineBuffer)
				for _, line := range lines {
					line = strings.TrimPrefix(line, "OK ")
					u.parseLine(line)
				}
			}
		}
	}()

	return ok
}

func (u *UDCO2S) StopMeasurement() {
	u.port.Write([]byte("STP\r\n"))
}

func (u *UDCO2S) queryFRCValue() {
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

func (u *UDCO2S) GetDeviceID() string {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.deviceID
}

func (u *UDCO2S) GetFirmwareVersion() string {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.version
}

func (u *UDCO2S) GetFRCValue() int {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.frcValue
}

func (u *UDCO2S) GetTimestamp() time.Time {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.timestamp
}

func (u *UDCO2S) GetCO2Value() int {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.co2Value
}

func (u *UDCO2S) GetHumidityValue() float64 {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.humidity
}

func (u *UDCO2S) GetTemperatureValue() float64 {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.temperature
}
