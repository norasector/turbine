package fsk4

import (
	"math"
)

type FSK4Demodulator struct {
	sampleRate int
	symbolRate int

	blockRate   float32
	history     [kNumTaps]float32
	historyLast int

	symbolClock  float32
	symbolSpread float32
	symbolTime   float32

	fineFreqCorrection   float32
	coarseFreqCorrection float32

	bfsk bool
}

func NewFSK4Demodulator(sampleRate, symbolRate int, bfsk bool) *FSK4Demodulator {
	ret := &FSK4Demodulator{
		sampleRate:           sampleRate,
		symbolRate:           symbolRate,
		blockRate:            float32(sampleRate) / float32(symbolRate),
		historyLast:          0,
		symbolClock:          0.0,
		symbolSpread:         defaultSymbolSpread,
		symbolTime:           float32(symbolRate) / float32(sampleRate),
		fineFreqCorrection:   0.0,
		coarseFreqCorrection: 0.0,
		bfsk:                 bfsk,
	}

	return ret
}

func (f *FSK4Demodulator) trackingLoopMMSE(input float32, output *float32) bool {
	f.symbolClock += f.symbolTime
	f.history[f.historyLast] = input
	f.historyLast++
	f.historyLast %= kNumTaps
	// f.historyLast = (f.historyLast + 1) % kNumTaps

	if f.symbolClock <= 1.0 {
		return false
	}

	f.symbolClock -= 1.0
	// at this point we state that linear interpolation was tried
	// but found to be slightly inferior.  Using MMSE
	// interpolation shouldn't be a terrible burden
	var imu int = int(math.Floor(float64(0.5 + (float32(kNumSteps) * (f.symbolClock / f.symbolTime)))))
	var imu_p1 int = imu + 1

	if imu >= kNumSteps {
		imu = kNumSteps - 1
		imu_p1 = kNumSteps
	}

	j := f.historyLast
	var interp float32 = 0.0
	var interp_p1 float32 = 0.0
	for i := 0; i < kNumTaps; i++ {
		interp += taps[imu][i] * f.history[j]
		interp_p1 += taps[imu_p1][i] * f.history[j]
		j = (j + 1) % kNumTaps
	}

	// our output symbol will be interpolated value corrected for
	// symbol_spread and frequency offset
	interp -= f.fineFreqCorrection
	interp_p1 -= f.fineFreqCorrection

	// output is corrected for symbol deviation (spread)
	*output = float32(2.0 * interp / f.symbolSpread)

	// detect received symbol error: basically use a hard decision
	// and subtract off expected position nominal symbol level
	// which will be +/- 0.5 * symbol_spread and +/- 1.5 *
	// symbol_spread remember: nominal symbol_spread will be 2.0
	var symbolError float32

	if f.bfsk {
		if interp < 0.0 {
			// symbol is -1: Expected at -0.5 * symbol_spread
			symbolError = interp + (0.5 * f.symbolSpread)
			f.symbolSpread -= (symbolError * kSymbolSpread)
		} else {
			// symbol is +1: Expected at +0.5 * symbol_spread
			symbolError = interp - (0.5 * f.symbolSpread)
			f.symbolSpread += (symbolError * kSymbolSpread)
		}
	} else { // !f.bfsk
		if interp < -f.symbolSpread {
			// symbol is -3: Expected at -1.5 * symbol_spread
			symbolError = interp + (1.5 * f.symbolSpread)
			f.symbolSpread -= (symbolError * 0.5 * kSymbolSpread)
		} else if interp < 0.0 {
			// symbol is -1: Expected at -0.5 * symbol_spread
			symbolError = interp + (0.5 * f.symbolSpread)
			f.symbolSpread -= (symbolError * kSymbolSpread)
		} else if interp < f.symbolSpread {
			// symbol is +1: Expected at +0.5 * symbol_spread
			symbolError = interp - (0.5 * f.symbolSpread)
			f.symbolSpread += (symbolError * kSymbolSpread)
		} else {
			// symbol is +3: Expected at +1.5 * symbol_spread
			symbolError = interp - (1.5 * f.symbolSpread)
			f.symbolSpread += (symbolError * 0.5 * kSymbolSpread)
		}
	}

	if interp_p1 < interp {
		f.symbolClock += (symbolError * kSymbolTiming)
	} else {
		f.symbolClock -= (symbolError * kSymbolTiming)
	}

	f.symbolSpread = float32(math.Max(float64(f.symbolSpread), kSymbolSpreadMin))
	f.symbolSpread = float32(math.Min(float64(f.symbolSpread), kSymbolSpreadMax))

	f.coarseFreqCorrection += ((f.fineFreqCorrection - f.coarseFreqCorrection) * kCoarseFrequency)
	f.fineFreqCorrection += (symbolError * kFineFrequency)

	return true
}

func (f *FSK4Demodulator) WorkBuffer(inputItems, outputItems []float32) int {
	n := 0
	for i := 0; i < len(inputItems); i++ {
		if f.trackingLoopMMSE(inputItems[i], &outputItems[n]) {
			n++
		}
	}

	return n
}

func (f *FSK4Demodulator) Work(inputItems []float32) []float32 {
	outputItems := make([]float32, len(inputItems))

	length := f.WorkBuffer(inputItems, outputItems)

	return outputItems[:length]
}

func (f *FSK4Demodulator) Reset() {
	f.coarseFreqCorrection = 0.0
	f.fineFreqCorrection = 0.0
	f.symbolClock = 0.0
	f.symbolSpread = defaultSymbolSpread
}

func (f *FSK4Demodulator) PredictOutputSize(inputSize int) int {
	return int(float64(f.sampleRate)/float64(f.symbolRate)) * inputSize
}
