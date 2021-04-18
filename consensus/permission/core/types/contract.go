package types

import (
	"crypto/ecdsa"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/permission/core"
)

// TxArgs holds arguments required for execute functions
type TxArgs struct {
	Url        string
	AcctId     common.Address
	AccessType uint8
	Action     uint8
	Txa        ethapi.SendTxArgs
}

type ContractBackend struct {
	EthClnt     bind.ContractBackend
	Key         *ecdsa.PrivateKey
	PermConfig  *PermissionConfig
	IsAuthority bool
	UseDns      bool
}

// Node services
type NodeService interface {
	AddNode(_args TxArgs) (*types.Transaction, error)
	UpdateNodeStatus(_args TxArgs) (*types.Transaction, error)
	StartBlacklistedNodeRecovery(_args TxArgs) (*types.Transaction, error)
	ApproveBlacklistedNodeRecovery(_args TxArgs) (*types.Transaction, error)
}

// Control services
type ControlService interface {
	ConnectionAllowed(_enodeId, _ip string, _port, _raftPort uint16) (bool, error)
	TransactionAllowed(_sender common.Address, _target common.Address, _value *big.Int, _gasPrice *big.Int, _gasLimit *big.Int, _payload []byte, _transactionType core.TransactionType) error
}

type InitService interface {
	BindContracts() error
	Init(_breadth *big.Int, _depth *big.Int) (*types.Transaction, error)
	UpdateNetworkBootStatus() (*types.Transaction, error)
	SetPolicy(_nwAdminOrg string, _nwAdminRole string, _oAdminRole string) (*types.Transaction, error)
	GetNetworkBootStatus() (bool, error)

	AddAdminNode(url string) (*types.Transaction, error)

	GetNodeDetailsFromIndex(_nodeIndex *big.Int) (string, string, *big.Int, error)
	GetNumberOfNodes() (*big.Int, error)
	GetNodeDetails(enodeId string) (string, string, *big.Int, error)
}

func BindContract(contractInstance interface{}, bindFunc func() (interface{}, error)) error {
	element := reflect.ValueOf(contractInstance).Elem()
	instance, err := bindFunc()
	if err != nil {
		return err
	}
	element.Set(reflect.ValueOf(instance))
	return nil
}
