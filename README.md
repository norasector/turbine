# turbine

Turbine is the SDR software for [NoraSector](https://www.norasector.com/).  It's designed to capture and stream all frequencies in a trunked radio system.  It is capable of decoding multiple systems concurrently, even different system types, provided they all fall within the same sample bandwidth generated by the radio and there's enough CPU available.

It's built with the expectation that it uses a single SDR that is able to capture the bandwidth containing all frequencies in the system.

For instance, in the KCERS radio system, all frequencies fall within the 851-857MHz band.  Therefore, it uses a 8MSPS rate centered at 854MHz to capture 850-858MHz and thus all frequencies within that range can be processed.  SERS in Snohomish county also falls entirely within this range, and the antenna can pick it up, so it can also be processed in the same instance of the program.

All audio is encoded using the Opus codec for compatibility with WebRTC and output over UDP.

Turbine borrows heavily from great projects such as [op25](https://osmocom.org/projects/op25/wiki) and [GNURadio](https://www.gnuradio.org/).  As these projects are licensed under the GPL, so too is Turbine, as it would be considered a derivative work.

Turbine is designed to be run on ample hardware and radios.  NoraSector's production radio runs turbine on a dedicated Intel i7-11700k and it consumes approximately 60% of all cores.  It currently uses a HackRF one, but there should be no issue with any other SDR provided it outputs IQ samples at an adequate sample rate.

## Dependencies

* `libopus-dev`
* `libopusfile-dev`
* `libhackrf-dev`
* `librtlsdr-dev` -- note that there is a RTLSDR driver, but it's mostly valuable for testing and debugging single frequencies.  Turbine doesn't support bonding multiple radios.

## Building

### Local

```
go build -o bin/turbine ./cmd/turbine
./bin/turbine turbine.yaml
```

### Docker

```
docker build -t turbine .

# Note that you must grant access to the device to Docker
# The following command will run the container and grant access to the SDR

DEVICE=$(lsusb | grep HackRF | awk '{printf "/dev/bus/usb/%s/%s",$2,substr($4, 1, length($4)-1)}');
docker run --network=host -v `pwd`/turbine.yaml:/app/turbine.yaml --rm --name turbine --device $DEVICE turbine
```

## Visualization server

Turbine comes with a built-in visualization server, hosted at `:3333` by default.

Once running, navigate to `http://localhost:3333` to access it.  You can then view graphs of the DSP processing stages of each frequency being processed.

![Screenshot of Turbine viz server](/images/viz-server.png)

To fit more graphs on the screen, just use Cmd/Ctrl+- to shrink down the size.


## Output format

The output format is defined [here](https://github.com/norasector/turbine-common).  Audio is encoded as Opus audio frames and wrapped in a small envelope with metadata such as system_id and tgid and then marshaled as protobuf before sending over the wire.

## Supported systems

* Motorola SmartZone

## TODO:

* Finish porting P25 stuff
* Factor out recording functionality into another binary
* Figure out what to do with segdsp fork -- currently it's being overridden in the go.mod file