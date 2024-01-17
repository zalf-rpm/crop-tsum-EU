package main

// calculate harvest rain
// from SoybeanEU project

// ring buffer for last days
type dataLastDays struct {
	arr        []float64
	index      int
	currentLen int
	capacity   int
}

func newDataLastDays(days int) dataLastDays {
	return dataLastDays{arr: make([]float64, days), index: 0, capacity: days}
}

func (d *dataLastDays) addDay(val float64) {
	if d.index < d.capacity-1 {
		d.index++
		if d.currentLen < d.capacity {
			d.currentLen++
		}
	} else {
		d.index = 0
	}
	d.arr[d.index] = val
}

func (d *dataLastDays) getData() []float64 {
	if d.currentLen == 0 {
		return nil
	}
	// return an array, starting with the oldest entry
	rArr := make([]float64, d.currentLen)
	hIndex := d.index
	for i := 0; i < d.currentLen; i++ {
		if hIndex < d.currentLen-1 {
			hIndex++
		} else {
			hIndex = 0
		}
		rArr[i] = d.arr[hIndex]
	}
	return rArr
}
