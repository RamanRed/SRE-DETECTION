package service

import (
	"crypto/rand"
	"fmt"
	"time"
)

type Clock func() time.Time
type IDGenerator func() string

func SystemClock() time.Time {
	return time.Now()
}

func UUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate UUID: %v", err))
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func stringPointer(value string) *string {
	return &value
}

func intPointer(value int) *int {
	return &value
}
