package abi

//nolint:golint
import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed amb.json
var arbitraryMessageJSONABI string

//go:embed erc_to_native.json
var ercToNativeJSONABI string

var (
	ArbitraryMessageABI abi.ABI
	ErcToNativeABI      abi.ABI
)

const (
	UserRequestForSignature         = "event UserRequestForSignature(bytes32 indexed messageId, bytes encodedData)"
	LegacyUserRequestForSignature   = "event UserRequestForSignature(bytes encodedData)"
	UserRequestForAffirmation       = "event UserRequestForAffirmation(bytes32 indexed messageId, bytes encodedData)"
	LegacyUserRequestForAffirmation = "event UserRequestForAffirmation(bytes encodedData)"
	UserRequestForInformation       = "event UserRequestForInformation(bytes32 indexed messageId, bytes32 indexed requestSelector, address indexed sender, bytes data)"
	SignedForUserRequest            = "event SignedForUserRequest(address indexed signer, bytes32 messageHash)"
	SignedForAffirmation            = "event SignedForAffirmation(address indexed signer, bytes32 messageHash)"
	SignedForInformation            = "event SignedForInformation(address indexed signer, bytes32 indexed messageId)"
	CollectedSignatures             = "event CollectedSignatures(address authorityResponsibleForRelay, bytes32 messageHash, uint256 NumberOfCollectedSignatures)"
	AffirmationCompleted            = "event AffirmationCompleted(address indexed sender, address indexed executor, bytes32 indexed messageId, bool status)"
	LegacyAffirmationCompleted      = "event AffirmationCompleted(address sender, address executor, bytes32 messageId, bool status)"
	RelayedMessage                  = "event RelayedMessage(address indexed sender, address indexed executor, bytes32 indexed messageId, bool status)"
	LegacyRelayedMessage            = "event RelayedMessage(address sender, address executor, bytes32 messageId, bool status)"
	InformationRetrieved            = "event InformationRetrieved(bytes32 indexed messageId, bool status, bool callbackStatus)"

	ErcToNativeUserRequestForSignature   = "event UserRequestForSignature(address recipient, uint256 value)"
	ErcToNativeTransfer                  = "event Transfer(address indexed from, address indexed to, uint256 value)"
	ErcToNativeRelayedMessage            = "event RelayedMessage(address recipient, uint256 value, bytes32 transactionHash)"
	ErcToNativeUserRequestForAffirmation = "event UserRequestForAffirmation(address recipient, uint256 value)"
	ErcToNativeAffirmationCompleted      = "event AffirmationCompleted(address recipient, uint256 value, bytes32 transactionHash)"
	ErcToNativeSignedForAffirmation      = "event SignedForAffirmation(address indexed signer, bytes32 transactionHash)"

	ValidatorAdded   = "event ValidatorAdded(address indexed validator)"
	ValidatorRemoved = "event ValidatorRemoved(address indexed validator)"
)

func init() {
	var err error
	ArbitraryMessageABI, err = abi.JSON(strings.NewReader(arbitraryMessageJSONABI))
	if err != nil {
		panic(err)
	}
	ErcToNativeABI, err = abi.JSON(strings.NewReader(ercToNativeJSONABI))
	if err != nil {
		panic(err)
	}
}
