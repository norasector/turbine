package viz

import (
	"bytes"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type PlotType int

const (
	PlotTypeDefault PlotType = iota
	PlotTypeScatter
	PlotTypeLines
)

type TimeDomainPlotter struct {
	bufFloat    []float32
	size        int
	name        string
	plotFunc    func(*plot.Plot, ...interface{}) error
	plotOptions []PlotOptions
}

func NewTimeDomainPlotter(name string, size int) *TimeDomainPlotter {
	ret := &TimeDomainPlotter{
		bufFloat: make([]float32, 0),
		size:     size,
		name:     name,
		plotFunc: plotutil.AddScatters,
	}

	return ret
}

func (t *TimeDomainPlotter) Name() string {
	return t.name
}

func (t *TimeDomainPlotter) SetPlotType(tp PlotType) {
	switch tp {
	case PlotTypeLines:
		t.plotFunc = plotutil.AddLines
	default:
		t.plotFunc = plotutil.AddScatters
	}

}

func (tp *TimeDomainPlotter) AppendFloat(f []float32) {
	tp.bufFloat = append(tp.bufFloat, f...)

	if len(tp.bufFloat) > tp.size {
		tp.bufFloat = tp.bufFloat[len(tp.bufFloat)-tp.size:]
	}

	// if time.Since(tp.lastEmitTime) > refreshTime {
	// 	tp.lastEmitTime = time.Now()
	// 	go tp.PlotOut()
	// }
}

func (tp *TimeDomainPlotter) AddPlotOption(opt PlotOptions) {
	tp.plotOptions = append(tp.plotOptions, opt)
}

func (tp *TimeDomainPlotter) GetImage() *ImageContainer {
	if len(tp.bufFloat) < tp.size {
		return nil
	}

	p := plotWithDefaults()

	p.Title.Text = tp.name
	p.Y.Label.Text = "Amplitude"
	p.Y.Min = -4
	p.Y.Max = 4
	p.X.Label.Text = "t"

	for _, opt := range tp.plotOptions {
		opt(p)
	}

	grid := plotter.NewGrid()
	p.Add(grid)

	tp.plotFunc(p, "f(t)", func() plotter.XYs {

		ret := make(plotter.XYs, tp.size)
		for i := 0; i < tp.size; i++ {

			ret[i] = plotter.XY{X: float64(i), Y: float64(tp.bufFloat[i])}
		}

		return ret
	}())

	var imageData bytes.Buffer
	w, err := p.WriterTo(8*vg.Inch, 6*vg.Inch, "png")
	if err != nil {
		panic(err)
	}
	w.WriteTo(&imageData)
	return &ImageContainer{name: tp.name, data: imageData.Bytes()}
}
