/// Author: Bernhard Tittelbach, btittelbach@github  (c) 2015

package bbhw

type MMappedGPIOInCollection struct {
	MMappedGPIO
	collection *MMappedGPIOCollectionFactory
}

type MMappedGPIOCollectionFactory struct {
	//4 32bit arrays to be copied to register
	gpios_to_set   []uint32
	gpios_to_clear []uint32
	record_changes bool
}

/// ---------- MMappedGPIOCollectionFactory ---------------

func NewMMapedGPIOCollectionFactory() (gpiocf *MMappedGPIOCollectionFactory) {
	mmapreg := getgpiommap()
	gpiocf = new(MMappedGPIOCollectionFactory)
	gpiocf.gpios_to_set = make([]uint32, len(mmapreg.memgpiochipreg32))
	gpiocf.gpios_to_clear = make([]uint32, len(mmapreg.memgpiochipreg32))
	return gpiocf
}

func (gpiocf *MMappedGPIOCollectionFactory) EndTransactionApplySetStates() {
	mmapreg := getgpiommap()
	for i, _ := range gpiocf.gpios_to_set {
		mmapreg.memgpiochipreg32[i][intgpio_setdataout_o32_] = gpiocf.gpios_to_set[i]
		mmapreg.memgpiochipreg32[i][intgpio_cleardataout_o32_] = gpiocf.gpios_to_clear[i]
		gpiocf.gpios_to_set[i] = 0
		gpiocf.gpios_to_clear[i] = 0
	}
	gpiocf.record_changes = false
}

func (gpiocf *MMappedGPIOCollectionFactory) BeginTransactionRecordSetStates() {
	gpiocf.record_changes = true
}

func (gpiocf *MMappedGPIOCollectionFactory) NewMMapedGPIO(number uint, direction int) (gpio *MMappedGPIOInCollection) {
	NewSysfsGPIOOrPanic(number, direction).Close()
	gpio = new(MMappedGPIOInCollection)
	gpio.chipid, gpio.gpioid = calcGPIOAddrFromLinuxGPIONum(number)
	gpio.collection = gpiocf
	return gpio
}

/// ------------- MMappedGPIOInCollection Methods -------------------

func (gpio *MMappedGPIOInCollection) SetStateNow(state bool) error {
	return gpio.MMappedGPIO.SetState(state)
}

func (gpio *MMappedGPIOInCollection) SetFutureState(state bool) error {
	if state {
		gpio.collection.gpios_to_set[gpio.chipid] |= uint32(1 << gpio.gpioid)
		gpio.collection.gpios_to_clear[gpio.chipid] &= ^uint32(1 << gpio.gpioid)
	} else {
		gpio.collection.gpios_to_clear[gpio.chipid] |= uint32(1 << gpio.gpioid)
		gpio.collection.gpios_to_set[gpio.chipid] &= ^uint32(1 << gpio.gpioid)
	}
	return nil
}

/// Checks if State was Set during a transaction but not yet applied
/// state_known returns true if state was set
/// state returns the future state
/// err returns nil
func (gpio *MMappedGPIOInCollection) GetFutureState() (state_known, state bool, err error) {
	state = gpio.collection.gpios_to_set[gpio.chipid]&uint32(1<<gpio.gpioid) > 0
	state_known = state
	if !state_known {
		state_known = gpio.collection.gpios_to_clear[gpio.chipid]&uint32(1<<gpio.gpioid) > 0
	}
	return
}

func (gpio *MMappedGPIOInCollection) SetState(state bool) error {
	if gpio.collection.record_changes {
		return gpio.SetFutureState(state)
	} else {
		return gpio.SetStateNow(state)
	}
}
