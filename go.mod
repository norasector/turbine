module github.com/norasector/turbine

go 1.17

require (
	github.com/hraban/opus v0.0.0-20211030232353-1a9beeaf0764
	github.com/influxdata/influxdb-client-go v1.4.0
	github.com/jpoirier/gortlsdr v2.10.0+incompatible
	github.com/julienschmidt/httprouter v1.3.0
	github.com/mjibson/go-dsp v0.0.0-20180508042940-11479a337f12
	github.com/norasector/turbine-common v0.0.0-20220108182241-6fe1c1be582b
	github.com/racerxdl/segdsp v0.0.0-20190825170906-a855d00a24a8
	github.com/rs/zerolog v1.26.1
	github.com/samuel/go-hackrf v0.0.0-20171108215759-68a81b40b34d
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gonum.org/v1/gonum v0.9.3
	gonum.org/v1/plot v0.10.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/ajstarks/svgo v0.0.0-20210923152817-c3b6e2f0c527 // indirect
	github.com/deepmap/oapi-codegen v1.3.6 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/go-fonts/liberation v0.2.0 // indirect
	github.com/go-latex/latex v0.0.0-20210823091927-c0d11ff05a81 // indirect
	github.com/go-pdf/fpdf v0.5.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/influxdata/line-protocol v0.0.0-20200327222509-2487e7298839 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/labstack/echo/v4 v4.1.11 // indirect
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.1.0 // indirect
	golang.org/x/crypto v0.0.0-20211215165025-cf75a172585e // indirect
	golang.org/x/image v0.0.0-20210628002857-a66eb6448b8d // indirect
	golang.org/x/net v0.0.0-20210805182204-aaa1db679c0d // indirect
	golang.org/x/sys v0.0.0-20210809222454-d867a43fc93e // indirect
	golang.org/x/text v0.3.6 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
)

replace github.com/racerxdl/segdsp v0.0.0-20190825170906-a855d00a24a8 => github.com/ap0/segdsp v0.1.3
