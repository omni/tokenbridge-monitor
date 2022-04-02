package constants

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed amb.json
var ambJsonABI string

//go:embed erc_to_native.json
var etnJsonABI string

var AMB, ERC_TO_NATIVE abi.ABI

func init() {
	var err error
	AMB, err = abi.JSON(strings.NewReader(ambJsonABI))
	if err != nil {
		panic(err)
	}
	ERC_TO_NATIVE, err = abi.JSON(strings.NewReader(etnJsonABI))
	if err != nil {
		panic(err)
	}
}
