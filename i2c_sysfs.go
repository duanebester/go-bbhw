package bbhw

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// flags
const (
	flagTEN        = 0x0010 // this is a ten bit chip address
	flagRD         = 0x0001 // read data, from slave to master
	flagSTOP       = 0x8000 // if funcProtocolMangling
	flagNOSTART    = 0x4000 // if I2C_FUNC_NOSTART
	flagRevDirAddr = 0x2000 // if funcProtocolMangling
	flagIgnoreNAK  = 0x1000 // if funcProtocolMangling
	flagNoRDACK    = 0x0800 // if funcProtocolMangling
	flagRecvLen    = 0x0400 // length will be first received byte
)

// i2cdev driver IOCTL control codes.
//
// Constants and structure definition can be found at
// /usr/include/linux/i2c-dev.h and /usr/include/linux/i2c.h.
const (
	ioctlRetries = 0x701
	ioctlTimeout = 0x702
	ioctlSlave   = 0x703
	ioctlTenBits = 0x704
	ioctlFuncs   = 0x705
	ioctlRdwr    = 0x707
)

type SysfsI2C struct {
	Bus int
	fd  *os.File
	mu  sync.Mutex
}

type i2cMsg struct {
	addr   uint16 // Address to communicate with
	flags  uint16 // 1 for read, see i2c.h for more details
	length uint16
	buf    uintptr
}

type rdwrIoctlData struct {
	msgs  uintptr // Pointer to i2cMsg
	nmsgs uint32
}

func NewI2C(bus int) (*SysfsI2C, error) {
	i2c := new(SysfsI2C)
	i2c.Bus = bus
	fd, err := os.OpenFile(fmt.Sprintf("/dev/i2c-%d", bus), os.O_RDWR|os.O_SYNC, 0666)
	if err != nil {
		return nil, err
	}
	i2c.fd = fd
	return i2c, nil
}

func NewSysfsI2COrPanic(bus int) *SysfsI2C {
	i2c, err := NewI2C(bus)
	if err != nil {
		panic(err)
	}
	return i2c
}

// Tx execute a transaction as a single operation unit.
func (i2c *SysfsI2C) Tx(addr uint16, w, r []byte) error {
	if addr >= 0x400 /*|| (addr >= 0x80 && i.fn&func10BitAddr == 0)*/ {
		return errors.New("sysfs-i2c: invalid address")
	}

	if len(w) == 0 && len(r) == 0 {
		return nil
	}

	// Convert the messages to the internal format.
	var buf [2]i2cMsg
	msgs := buf[0:0]
	if len(w) != 0 {
		msgs = buf[:1]
		buf[0].addr = addr
		buf[0].length = uint16(len(w))
		buf[0].buf = uintptr(unsafe.Pointer(&w[0]))
	}
	if len(r) != 0 {
		l := len(msgs)
		msgs = msgs[:l+1] // extend the slice by one
		buf[l].addr = addr
		buf[l].flags = flagRD
		buf[l].length = uint16(len(r))
		buf[l].buf = uintptr(unsafe.Pointer(&r[0]))
	}
	p := rdwrIoctlData{
		msgs:  uintptr(unsafe.Pointer(&msgs[0])),
		nmsgs: uint32(len(msgs)),
	}
	pp := uintptr(unsafe.Pointer(&p))
	i2c.mu.Lock()
	defer i2c.mu.Unlock()
	if err := ioctl(i2c.fd.Fd(), ioctlRdwr, pp); err != nil {
		return fmt.Errorf("sysfs-i2c: %v", err)
	}
	return nil
}

// Write writes to the I²C bus without reading, implementing io.Writer.
//
// It's a wrapper for Tx()
func (i2c *SysfsI2C) Write(addr uint16, b []byte) (int, error) {
	if err := i2c.Tx(addr, b, nil); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (i2c *SysfsI2C) Close() error {
	if i2c == nil {
		return errors.New("i2c == nil")
	}
	if i2c.fd == nil {
		return errors.New("i2c.fd == nil")
	}
	i2c.mu.Lock()
	defer i2c.mu.Unlock()
	return i2c.fd.Close()
}

func ioctl(f uintptr, op uint, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f, uintptr(op), arg); errno != 0 {
		return syscall.Errno(errno)
	}
	return nil
}
