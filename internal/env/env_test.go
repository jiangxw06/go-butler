package env

import (
	"fmt"
	"testing"
)

func TestGetIPv4(t *testing.T) {
	fmt.Println(getOutBoundIP())
}
