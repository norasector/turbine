package config

import (
	"time"

	"github.com/norasector/turbine/pkg/op25"
)

type Config struct {
	CenterFreq            int                 `yaml:"center_freq"`
	SampleRate            int                 `yaml:"sample_rate"`
	VoiceSampleOutputRate int                 `yaml:"output_rate"`
	Systems               []System            `yaml:"systems"`
	OutputDestinations    []OutputDestination `yaml:"output_destinations"`
	FrequencyTimeout      time.Duration       `yaml:"freq_timeout"`
	RecordLocation        string              `yaml:"record_location"`
	PlaybackLocation      string              `yaml:"playback_location"`
	Device                string              `yaml:"device"`
	RTLSDRDeviceIndex     int                 `yaml:"rtlsdr_device_index"`
	VizServer             struct {
		Port           int           `yaml:"port"`
		UpdateInterval time.Duration `yaml:"update_interval_ms"`
	} `yaml:"viz_server"`
	InfluxDB struct {
		Host         string `yaml:"host"`
		Organization string `yaml:"organization"`
		Bucket       string `yaml:"bucket"`
	}
}

type OutputDestination struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type System struct {
	ID                 int             `yaml:"id"`
	Name               string          `yaml:"name"`
	ControlFrequencies []int           `yaml:"control_freqs,flow"`
	SystemType         op25.SystemType `yaml:"system_type"`
	SymbolRate         int             `yaml:"symbol_rate"`
	VoiceBandwidth     int             `yaml:"voice_bandwidth"`
	SquelchLevel       int             `yaml:"squelch_level"`
}
