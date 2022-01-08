package processor

import "github.com/norasector/turbine/pkg/dsp/viz"

type DataType int

const (
	DataTypeComplex DataType = iota
	DataTypeFloat
	DataTypeBytes
	DataTypeInt16
)

type DSPWorker struct {
	Name        string
	DisplayName string
	InputRate   int
	OutputRate  int

	inputDataType  DataType
	outputDataType DataType

	ccWorker CCWorker
	cfWorker CFWorker
	fbWorker FBWorker
	ffWorker FFWorker
	bbWorker BBWorker

	fOutputBuffer []float32
	cOutputBuffer []complex64
	bOutputBuffer []byte

	fft         *viz.FFTPlotter
	timeDomain  *viz.TimeDomainPlotter
	vizSize     int
	plotType    viz.PlotType
	floatFFT    bool
	showBalance bool

	plotOptions []viz.PlotOptions
}

type DSPWorkerOption func(r *DSPWorker)

func WithPlotOptions(opts []viz.PlotOptions) DSPWorkerOption {
	return func(r *DSPWorker) {
		r.plotOptions = append(r.plotOptions, opts...)
	}
}
func WithVizLength(length int) DSPWorkerOption {
	return func(r *DSPWorker) {
		r.vizSize = length
	}
}
func WithPlotType(plotType viz.PlotType) DSPWorkerOption {
	return func(r *DSPWorker) {
		r.plotType = plotType
	}
}

func WithFloatFFTPlot() DSPWorkerOption {
	return func(r *DSPWorker) {
		r.floatFFT = true
	}
}

func ShowFFTBalance() DSPWorkerOption {
	return func(r *DSPWorker) {
		r.showBalance = true
	}
}

func baseWorker(name, displayName string, inputRate, outputRate int) *DSPWorker {
	return &DSPWorker{
		Name:        name,
		DisplayName: displayName,
		InputRate:   inputRate,
		OutputRate:  outputRate,
	}
}

func NewDSPWorkerCC(name, displayName string, inputRate, outputRate int, worker CCWorker, opts ...DSPWorkerOption) *DSPWorker {
	ret := baseWorker(name, displayName, inputRate, outputRate)
	ret.inputDataType = DataTypeComplex
	ret.outputDataType = DataTypeComplex
	ret.ccWorker = worker

	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

func NewDSPWorkerCF(name, displayName string, inputRate, outputRate int, worker CFWorker, opts ...DSPWorkerOption) *DSPWorker {
	ret := baseWorker(name, displayName, inputRate, outputRate)
	ret.inputDataType = DataTypeComplex
	ret.outputDataType = DataTypeFloat
	ret.cfWorker = worker

	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

func NewDSPWorkerFF(name, displayName string, inputRate, outputRate int, worker FFWorker, opts ...DSPWorkerOption) *DSPWorker {
	ret := baseWorker(name, displayName, inputRate, outputRate)
	ret.inputDataType = DataTypeFloat
	ret.outputDataType = DataTypeFloat
	ret.ffWorker = worker

	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

func NewDSPWorkerFB(name, displayName string, inputRate, outputRate int, worker FBWorker, opts ...DSPWorkerOption) *DSPWorker {
	ret := baseWorker(name, displayName, inputRate, outputRate)
	ret.inputDataType = DataTypeFloat
	ret.outputDataType = DataTypeBytes
	ret.fbWorker = worker

	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

// Complex in, complex out
type CCWorker interface {
	WorkBuffer([]complex64, []complex64) int
	PredictOutputSize(int) int
}

// Complex in, float out
type CFWorker interface {
	WorkBuffer([]complex64, []float32) int
	PredictOutputSize(int) int
}

// Float in, binary bytes out (1 symbol per byte)
type FBWorker interface {
	WorkBuffer([]float32, []byte) int
	PredictOutputSize(int) int
}

type FFWorker interface {
	WorkBuffer([]float32, []float32) int
	PredictOutputSize(int) int
}

type BBWorker interface {
	WorkBuffer([]byte, []byte) int
	PredictOutputSize(int) int
}
