package processor

import (
	"errors"
	"fmt"
	"time"

	"github.com/norasector/turbine-common/types"
	"github.com/norasector/turbine/pkg/dsp/viz"
)

type Processor struct {
	Name        string
	InputName   string
	blocks      []*DSPWorker
	vizServer   *viz.Server
	initialized bool
	inputFFT    *viz.FFTPlotter
}

func NewProcessor(name, inputName string, vizServer *viz.Server) *Processor {
	ret := &Processor{
		Name:      name,
		InputName: inputName,
		vizServer: vizServer,
	}

	return ret
}

func (p *Processor) AddBlock(worker *DSPWorker) {
	p.blocks = append(p.blocks, worker)
}

func (p *Processor) Initialize() error {
	if p.initialized {
		return nil
	}
	if len(p.blocks) < 2 {
		return fmt.Errorf("must specify at least 2 blocks")
	}
	cur := p.blocks[0]

	vizIndex := 0
	nextIndexString := func(s string) string {
		vizIndex++
		return fmt.Sprintf("%02d. %s", vizIndex, s)
	}

	p.inputFFT = viz.NewFFTPlotterComplex(nextIndexString(p.InputName), 1024, cur.InputRate)
	p.vizServer.Register(p.Name, p.inputFFT)

	for i := 1; i < len(p.blocks); i++ {
		next := p.blocks[i]

		if cur.outputDataType != next.inputDataType {
			return fmt.Errorf("cur: %s next %s data type mismatch (%d %d)", cur.Name, next.Name, cur.outputDataType, next.inputDataType)
		}
		if cur.OutputRate != next.InputRate {
			return fmt.Errorf("cur: %s next %s rate mismatch (%d %d)", cur.Name, next.Name, cur.OutputRate, next.InputRate)
		}

		switch cur.outputDataType {
		case DataTypeComplex:
			vizLength := 1024
			if cur.vizSize > 0 {
				vizLength = cur.vizSize
			}
			cur.fft = viz.NewFFTPlotterComplex(nextIndexString(cur.DisplayName), vizLength, cur.OutputRate)
			for _, opt := range cur.plotOptions {
				cur.fft.AddPlotOption(opt)
			}
			cur.fft.ShowBalance(cur.showBalance)
			p.vizServer.Register(p.Name, cur.fft)
		case DataTypeFloat:
			vizLength := 128
			if cur.vizSize > 0 {
				vizLength = cur.vizSize
			}
			cur.timeDomain = viz.NewTimeDomainPlotter(nextIndexString(cur.DisplayName), vizLength)
			for _, opt := range cur.plotOptions {
				cur.timeDomain.AddPlotOption(opt)
			}
			if cur.plotType != viz.PlotTypeDefault {
				cur.timeDomain.SetPlotType(cur.plotType)
			}
			p.vizServer.Register(p.Name, cur.timeDomain)

			v, ok := cur.ffWorker.(*viz.FFTFilter)
			if ok {
				v.FFTPlotterInput = viz.NewFFTPlotterFloat(nextIndexString(cur.DisplayName+" (Input FFT)"), 1024, cur.InputRate)
				p.vizServer.Register(p.Name, v.FFTPlotterInput)
				v.FFTPlotterOutput = viz.NewFFTPlotterFloat(nextIndexString(cur.DisplayName+" (Output FFT)"), 1024, cur.OutputRate)
				p.vizServer.Register(p.Name, v.FFTPlotterOutput)
			}
		}

		cur = next
	}

	switch cur.outputDataType {
	case DataTypeComplex:
		vizLength := 1024
		if cur.vizSize > 0 {
			vizLength = cur.vizSize
		}
		cur.fft = viz.NewFFTPlotterComplex(nextIndexString(cur.DisplayName), vizLength, cur.OutputRate)
		cur.fft.ShowBalance(cur.showBalance)
		for _, opt := range cur.plotOptions {
			cur.fft.AddPlotOption(opt)
		}
		p.vizServer.Register(p.Name, cur.fft)
	case DataTypeFloat:
		vizLength := 128
		if cur.vizSize > 0 {
			vizLength = cur.vizSize
		}
		cur.timeDomain = viz.NewTimeDomainPlotter(nextIndexString(cur.DisplayName), vizLength)
		if cur.plotType != viz.PlotTypeDefault {
			cur.timeDomain.SetPlotType(cur.plotType)
		}
		for _, opt := range cur.plotOptions {
			cur.timeDomain.AddPlotOption(opt)
		}
		p.vizServer.Register(p.Name, cur.timeDomain)

		v, ok := cur.ffWorker.(*viz.FFTFilter)
		if ok {
			v.FFTPlotterInput = viz.NewFFTPlotterFloat(nextIndexString(cur.DisplayName+" (Input FFT)"), 1024, cur.InputRate)
			p.vizServer.Register(p.Name, v.FFTPlotterInput)
			v.FFTPlotterOutput = viz.NewFFTPlotterFloat(nextIndexString(cur.DisplayName+" (Output FFT)"), 1024, cur.OutputRate)
			p.vizServer.Register(p.Name, v.FFTPlotterOutput)
		}
	}

	p.initialized = true

	return nil
}

// processData can handle arbitrary input and output types
func (p *Processor) processData(cmplxInput []complex64, floatInput []float32, byteInput []byte, expectedInputType, expectedOutputType DataType, metrics map[string]interface{}) ([]complex64, []float32, []byte, error) {
	cnt := 0
	if len(cmplxInput) > 0 {
		cnt++
	}
	if len(floatInput) > 0 {
		cnt++
	}
	if len(byteInput) > 0 {
		cnt++
	}
	if cnt == 0 {
		return nil, nil, nil, errors.New("must specify input")
	}
	if cnt > 1 {
		return nil, nil, nil, errors.New("may only specify one input")
	}

	var cmplxOutput []complex64
	var floatOutput []float32
	var byteOutput []byte

	if p.blocks[0].inputDataType != expectedInputType {
		return nil, nil, nil, fmt.Errorf("invalid input type: got %d expected %d", p.blocks[0].inputDataType, DataTypeComplex)
	}
	if p.blocks[len(p.blocks)-1].outputDataType != expectedOutputType {
		return nil, nil, nil, fmt.Errorf("invalid output type: got %d expected %d", p.blocks[len(p.blocks)-1].outputDataType, DataTypeBytes)
	}

	if cmplxInput != nil {
		p.inputFFT.AppendComplex(cmplxInput)
	}

	for _, block := range p.blocks {
		if block.inputDataType != expectedInputType {
			return nil, nil, nil, fmt.Errorf("error in %s: expected %d got %d input type", block.Name, expectedInputType, block.inputDataType)
		}

		var work func()

		switch block.inputDataType {
		case DataTypeComplex:
			switch block.outputDataType {
			case DataTypeComplex:

				if block.cOutputBuffer == nil {
					block.cOutputBuffer = make([]complex64, block.ccWorker.PredictOutputSize(len(cmplxInput))*2)
				}

				work = func() {
					length := block.ccWorker.WorkBuffer(cmplxInput, block.cOutputBuffer)
					cmplxOutput = block.cOutputBuffer[:length]

					if block.fft != nil {
						block.fft.AppendComplex(cmplxOutput)
					}
				}

			case DataTypeFloat:
				if block.fOutputBuffer == nil {
					block.fOutputBuffer = make([]float32, block.cfWorker.PredictOutputSize(len(cmplxInput))*2)
				}

				work = func() {

					length := block.cfWorker.WorkBuffer(cmplxInput, block.fOutputBuffer)
					floatOutput = block.fOutputBuffer[:length]

					if block.timeDomain != nil {
						block.timeDomain.AppendFloat(floatOutput)
					}
				}
			default:
				return nil, nil, nil, fmt.Errorf("%s unknown output type %d for input %d", block.Name, block.outputDataType, block.inputDataType)
			}

		case DataTypeFloat:
			switch block.outputDataType {
			case DataTypeFloat:
				if block.fOutputBuffer == nil || len(block.fOutputBuffer) != len(floatInput)*2 {
					block.fOutputBuffer = make([]float32, block.ffWorker.PredictOutputSize(len(floatInput))*2)
				}
				work = func() {

					length := block.ffWorker.WorkBuffer(floatInput, block.fOutputBuffer)
					floatOutput = block.fOutputBuffer[:length]
					if block.timeDomain != nil {
						block.timeDomain.AppendFloat(floatOutput)
					}
				}

			case DataTypeBytes:
				if block.bOutputBuffer == nil {
					block.bOutputBuffer = make([]byte, block.fbWorker.PredictOutputSize(len(floatInput))*2)
				}
				work = func() {
					length := block.fbWorker.WorkBuffer(floatInput, block.bOutputBuffer)
					byteOutput = block.bOutputBuffer[:length]
				}
			default:
				return nil, nil, nil, fmt.Errorf("%s unknown output type %d for input %d", block.Name, block.outputDataType, block.inputDataType)
			}

		case DataTypeBytes:
			switch block.outputDataType {
			case DataTypeBytes:
				if block.bOutputBuffer == nil {
					block.bOutputBuffer = make([]byte, block.fbWorker.PredictOutputSize(len(byteInput))*2)
				}
				work = func() {
					length := block.bbWorker.WorkBuffer(byteInput, block.bOutputBuffer)
					byteOutput = block.bOutputBuffer[:length]
				}
			default:
				return nil, nil, nil, fmt.Errorf("%s unknown output type %d for input %d", block.Name, block.outputDataType, block.inputDataType)

			}

		default:
			return nil, nil, nil, fmt.Errorf("unknown input type %d", block.inputDataType)
		}

		start := time.Now()
		work()
		metrics[fmt.Sprintf("%s_duration", block.Name)] = time.Since(start).Microseconds()

		if block != p.blocks[len(p.blocks)-1] {
			floatInput = floatOutput
			cmplxInput = cmplxOutput
			byteInput = byteOutput

			floatOutput = nil
			cmplxOutput = nil
			byteOutput = nil
			expectedInputType = block.outputDataType
		}
	}
	return cmplxOutput, floatOutput, byteOutput, nil
}

func (p *Processor) ProcessComplexToBinary(input *types.SegmentComplex64, metrics map[string]interface{}) (*types.SegmentBinaryBytes, error) {
	if !p.initialized {
		if err := p.Initialize(); err != nil {
			return nil, err
		}
	}

	_, _, byteOutput, err := p.processData(input.Data, nil, nil, DataTypeComplex, DataTypeBytes, metrics)
	if err != nil {
		return nil, err
	}

	return &types.SegmentBinaryBytes{
		SymbolRate:    p.blocks[len(p.blocks)-1].OutputRate,
		Data:          byteOutput,
		SegmentNumber: input.SegmentNumber,
	}, nil
}

func (p *Processor) ProcessComplexToFloat(input *types.SegmentComplex64, metrics map[string]interface{}) (*types.SegmentFloat32, error) {
	if !p.initialized {
		if err := p.Initialize(); err != nil {
			return nil, err
		}
	}

	_, floatOutput, _, err := p.processData(input.Data, nil, nil, DataTypeComplex, DataTypeFloat, metrics)
	if err != nil {
		return nil, err
	}

	return &types.SegmentFloat32{
		// Frequency: input.Frequency,
		SegmentNumber: input.SegmentNumber,
		Data:          floatOutput,
	}, nil
}
