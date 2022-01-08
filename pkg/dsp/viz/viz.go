package viz

import (
	"image/color"

	"gonum.org/v1/plot"
)

type PlotOptions func(p *plot.Plot)

func plotWithDefaults() *plot.Plot {

	p := plot.New()
	p.BackgroundColor = color.Black
	p.Title.TextStyle.Color = color.White
	p.Y.Label.TextStyle.Color = color.White
	p.Y.Color = color.White
	p.X.Label.TextStyle.Color = color.White
	p.X.Color = color.White
	p.Legend.TextStyle.Color = color.White
	p.X.Tick.Color = color.White
	p.Y.Tick.Color = color.White
	p.X.Tick.Label.Color = color.White
	p.Y.Tick.Label.Color = color.White

	return p
}
