package constants

import (
	_ "embed"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"strings"
)

//go:embed amb.json
var ambJsonABI string

var AMB abi.ABI

func init() {
	var err error
	AMB, err = abi.JSON(strings.NewReader(ambJsonABI))
	if err != nil {
		panic(err)
	}
}
