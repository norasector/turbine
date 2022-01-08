package viz

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/cmplx"

	"github.com/norasector/turbine/pkg/dsp/filters/fir"
	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

const (
	Y_AVG    = 0.03
	FFT_AVG  = 0.05
	MIX_AVG  = 0.10
	BAL_AVG  = 0.05
	FFT_BINS = 512  // number of fft bins
	FFT_FREQ = 0.05 // time interval between fft updates
	MIX_FREQ = 0.02 // time interval between mixer updates
)

type FFTPlotter struct {
	bufFloat     []float32
	bufComplex   []complex64
	sampleRate   int
	len          int
	isComplex    bool
	averagePower []float64
	avgSumPower  float64
	name         string
	// lastEmitTime time.Time
	showBalance bool
	plotOptions []PlotOptions
}

func (f *FFTPlotter) ShowBalance(show bool) {
	f.showBalance = show
}

func (f *FFTPlotter) Name() string {
	return f.name
}

func NewFFTPlotterFloat(name string, len, sampleRate int) *FFTPlotter {
	ret := &FFTPlotter{
		bufFloat:     make([]float32, len),
		averagePower: make([]float64, len),
		len:          len,
		sampleRate:   sampleRate,
		name:         name,
	}
	return ret
}
func NewFFTPlotterComplex(name string, len, sampleRate int) *FFTPlotter {
	ret := &FFTPlotter{
		bufComplex:   make([]complex64, len),
		averagePower: make([]float64, len),
		len:          len,
		sampleRate:   sampleRate,
		isComplex:    true,
		name:         name,
	}
	return ret
}

func (p *FFTPlotter) AppendFloat(s []float32) {
	if p.isComplex {
		panic(errors.New("wrong type float"))
	}
	if len(s) > p.len {
		p.bufFloat = s[len(s)-p.len:]
	} else {
		p.bufFloat = append(p.bufFloat, s...)
		p.bufFloat = p.bufFloat[len(s):]
	}
}

func (p *FFTPlotter) AppendComplex(s []complex64) {
	if !p.isComplex {
		panic(errors.New("wrong type complex"))
	}
	if len(s) > p.len {
		copy(p.bufComplex, s[len(s)-p.len:])
	} else {
		p.bufComplex = append(p.bufComplex, s...)
		p.bufComplex = p.bufComplex[len(s):]
	}
}

func (pb *FFTPlotter) AddPlotOption(opt PlotOptions) {
	pb.plotOptions = append(pb.plotOptions, opt)
}

func (pb *FFTPlotter) GetImage() *ImageContainer {

	p := plotWithDefaults()
	p.Title.Text = pb.name
	p.Y.Label.Text = "Power (dB)"
	p.X.Label.Text = "Frequency"
	// p.X.Max = 0
	// p.X.Min = -200

	p.Y.Max = 0
	p.Y.Min = -100

	for _, opt := range pb.plotOptions {
		opt(p)
	}

	grid := plotter.NewGrid()
	p.Add(grid)

	// ffted := fft.FFT(data)
	// fft.FFTS
	// log.Println("** DATA", len(data))

	var shiftFunc func(int) int
	var freqFunc func(int) float64
	var coeffs []complex128
	// taps := nsdsp.BlackmanHarrisWindow(pb.len, 67)
	win := fir.BlackmanWindow(pb.len)
	if pb.isComplex {
		f := fourier.NewCmplxFFT(pb.len)
		data := to128(pb.bufComplex)

		for i := 0; i < pb.len; i++ {
			data[i] = complex(real(data[i])*float64(win[i]), imag(data[i])*float64(win[i]))
			data[i] = data[i] / complex(0.42*float64(pb.len), 0.42*float64(pb.len))
		}

		coeffs = f.Coefficients(nil, data)
		shiftFunc = f.ShiftIdx
		freqFunc = f.Freq
	} else {
		f := fourier.NewFFT(pb.len)
		data := to64(pb.bufFloat)

		for i := 0; i < len(data); i++ {
			data[i] = data[i] * float64(win[i])
		}

		coeffs = f.Coefficients(nil, data)
		shiftFunc = func(i int) int { return i }
		freqFunc = f.Freq
	}

	plotutil.AddLines(p, "frequency", func() plotter.XYs {
		ret := make(plotter.XYs, len(coeffs))
		var sumPower float64
		for i := 0; i < len(coeffs); i++ {

			shiftIdx := shiftFunc(i)

			freq := freqFunc(shiftIdx) * float64(pb.sampleRate)
			mag := cmplx.Abs(coeffs[shiftIdx])

			pb.averagePower[i] = ((1.0 - MIX_AVG) * pb.averagePower[i]) + (MIX_AVG * mag)

			if pb.averagePower[i] == 0 {
				break
			}

			if pb.averagePower[i] > 1e-5 {
				if freq < 0 {
					sumPower -= pb.averagePower[i]
				} else if freq > 0 {
					sumPower += pb.averagePower[i]
				}
				pb.avgSumPower = ((1.0 - BAL_AVG) * pb.avgSumPower) + (BAL_AVG * sumPower)
			}
			ret[i] = plotter.XY{X: freq, Y: 20 * math.Log10(pb.averagePower[i])}
		}

		return ret
	}())

	if pb.showBalance {
		p.Title.Text += fmt.Sprintf(" Balance: %3.0f", math.Abs(pb.avgSumPower*1000))
	}

	var imageData bytes.Buffer
	w, err := p.WriterTo(8*vg.Inch, 6*vg.Inch, "png")
	if err != nil {
		panic(err)
	}
	w.WriteTo(&imageData)
	return &ImageContainer{name: pb.name, data: imageData.Bytes()}
	// p.Save(8*vg.Inch, 4*vg.Inch, name+".png")

}

func to128(c []complex64) []complex128 {
	ret := make([]complex128, len(c))
	for i := 0; i < len(c); i++ {
		ret[i] = complex128(c[i])
	}
	return ret
}
func to64(c []float32) []float64 {
	ret := make([]float64, len(c))
	for i := 0; i < len(c); i++ {
		ret[i] = float64(c[i])
	}
	return ret
}
