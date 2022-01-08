package util

import "math"

func FrequencyRange(freqs ...int) (low, high int) {
	low = math.MaxInt
	high = math.MinInt

	for _, freq := range freqs {
		if freq < low {
			low = freq
		}
		if freq > high {
			high = freq
		}
	}

	return
}

func CenterFrequencyAndSampleRate(low, high int) (center, bw int) {
	diff := high - low
	center = (low + high) / 2
	bw = diff / 2
	return
}

// Yes this is silly but I just want a standardized place for doing it.
func SampleRate(low, high int) int {
	return high - low
}
