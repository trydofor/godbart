package art

import (
	"fmt"
	"regexp"
	"testing"
)

func Test_Reg(t *testing.T) {

	fmt.Printf("%t\n", matchEntire(regexp.MustCompile("tx_parcle"), "tx_parcle_01"))
	fmt.Printf("%t\n", matchEntire(regexp.MustCompile("tx_parcle.*"), "tx_parcle_01"))
}
