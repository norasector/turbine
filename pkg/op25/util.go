package op25

import "fmt"

func MHzToString(mhz int) string {
	return fmt.Sprintf("%0.4f MHz", float64(mhz)/1e6)
}
