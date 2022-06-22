package utils

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func RestoreSignerAddress(data, sig []byte) (common.Address, error) {
	if len(sig) >= 65 && sig[64] >= 27 {
		sig[64] -= 27
	}
	pk, err := crypto.SigToPub(accounts.TextHash(data), sig)
	if err != nil {
		return common.Address{}, fmt.Errorf("can't recover ecdsa signer: %w", err)
	}
	return crypto.PubkeyToAddress(*pk), nil
}
