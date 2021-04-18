package core

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/hashicorp/golang-lru"
)

type TransactionType uint8

const (
	ValueTransferTxn TransactionType = iota
	ContractCallTxn
	ContractDeployTxn
)

type AccessType uint8

const (
	// common access type list for both V1 and V0 model.
	// the first 4 are used by both models
	// last 3 are used by V0 in alignment with EEA specs
	ReadOnly AccessType = iota
	Transact
	ContractDeploy
	FullAccess
	// below access types are only used by V0 model
	ContractCall
	TransactAndContractCall
	TransactAndContractDeploy
	ContractCallAndDeploy
)

type PermissionModelType uint8

const (
	V0 PermissionModelType = iota
	V1
	Default
)

type NodeStatus uint8

const (
	NodePendingApproval NodeStatus = iota + 1
	NodeApproved
	NodeDeactivated
	NodeBlackListed
	NodeRecoveryInitiated
)

type NodeInfo struct {
	OrgId  string     `json:"orgId"`
	Url    string     `json:"url"`
	Status NodeStatus `json:"status"`
}

var syncStarted = false

var defaultAccess = FullAccess
var qip714BlockReached = false
var networkBootUpCompleted = false
var networkAdminRole string
var orgAdminRole string
var PermissionModel = Default
var PermissionTransactionAllowedFunc func(_sender common.Address, _target common.Address, _value *big.Int, _gasPrice *big.Int, _gasLimit *big.Int, _payload []byte, _transactionType TransactionType) error
var (
	NodeInfoMap *NodeCache
)

type NodeKey struct {
	OrgId string
	Url   string
}

type NodeCache struct {
	c                       *lru.Cache
	evicted                 bool
	populateCacheFunc       func(string) (*NodeInfo, error)
	populateAndValidateFunc func(string, string) bool
}

func (n *NodeCache) PopulateValidateFunc(cf func(string, string) bool) {
	n.populateAndValidateFunc = cf
}

func (n *NodeCache) PopulateCacheFunc(cf func(string) (*NodeInfo, error)) {
	n.populateCacheFunc = cf
}

func NewNodeCache(cacheSize int) *NodeCache {
	nodeCache := NodeCache{evicted: false}
	onEvictedFunc := func(k interface{}, v interface{}) {
		nodeCache.evicted = true

	}
	nodeCache.c, _ = lru.NewWithEvict(cacheSize, onEvictedFunc)
	return &nodeCache
}

func SetSyncStatus() {
	syncStarted = true
}

func GetSyncStatus() bool {
	return syncStarted
}

// sets default access to read only
func setDefaultAccess() {
	if PermissionsEnabled() {
		defaultAccess = ReadOnly
	}
}

// sets the qip714block reached as true
func SetQIP714BlockReached() {
	qip714BlockReached = true
	setDefaultAccess()
}

// sets the network boot completed as true
func SetNetworkBootUpCompleted() {
	networkBootUpCompleted = true
	setDefaultAccess()
}

// return bool to indicate if permissions is enabled
func PermissionsEnabled() bool {
	if PermissionModel == V0 {
		return qip714BlockReached
	} else {
		return qip714BlockReached && networkBootUpCompleted
	}
}

func GetNodeUrl(enodeId string, ip string, port uint16, raftport uint16, isAuthority bool) string {
	if isAuthority {
		return fmt.Sprintf("enode://%s@%s:%d?discport=0&raftport=%d", enodeId, strings.Trim(ip, "\x00"), port, raftport)
	}
	return fmt.Sprintf("enode://%s@%s:%d?discport=0", enodeId, strings.Trim(ip, "\x00"), port)
}

func containsKey(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (n *NodeCache) UpsertNode(orgId string, url string, status NodeStatus) {
	key := NodeKey{OrgId: orgId, Url: url}
	n.c.Add(key, &NodeInfo{orgId, url, status})
}

func (n *NodeCache) GetNodeByUrl(url string) (*NodeInfo, error) {
	for _, k := range n.c.Keys() {
		ent := k.(NodeKey)
		if ent.Url == url {
			v, _ := n.c.Get(ent)
			return v.(*NodeInfo), nil
		}
	}
	// check if the node cache is evicted. if yes we need
	// fetch the record from the contract
	if n.evicted {

		// call cache population function to populate from contract
		nodeRec, err := n.populateCacheFunc(url)
		if err != nil {
			return nil, err
		}

		// insert the received record into cache
		n.UpsertNode(nodeRec.OrgId, nodeRec.Url, nodeRec.Status)
		//return the record
		return nodeRec, err
	}
	return nil, errors.New("Node does not exist")
}

func (n *NodeCache) GetNodeList() []NodeInfo {
	olist := make([]NodeInfo, len(n.c.Keys()))
	for i, k := range n.c.Keys() {
		v, _ := n.c.Get(k)
		vp := v.(*NodeInfo)
		olist[i] = *vp
	}
	return olist
}

// validates if the account can transact from the current node
func ValidateNodeForTxn(hexnodeId string, from common.Address) bool {
	if !PermissionsEnabled() || hexnodeId == "" {
		return true
	}

	passedEnodeId, err := enode.ParseV4(hexnodeId)
	if err != nil {
		return false
	}

	ac, _ := AcctInfoMap.GetAccount(from)
	if ac == nil {
		return true
	}

	acOrgRec, err := OrgInfoMap.GetOrg(ac.OrgId)
	if err != nil {
		return false
	}

	// scan through the node list and validate
	for _, n := range NodeInfoMap.GetNodeList() {
		orgRec, err := OrgInfoMap.GetOrg(n.OrgId)
		if err != nil {
			return false
		}
		if orgRec.UltimateParent == acOrgRec.UltimateParent {
			recEnodeId, _ := enode.ParseV4(n.Url)
			if recEnodeId.ID() == passedEnodeId.ID() && n.Status == NodeApproved {
				return true
			}
		}
	}
	if NodeInfoMap.evicted {
		return NodeInfoMap.populateAndValidateFunc(hexnodeId, acOrgRec.UltimateParent)
	}

	return false
}

func IsV0Permission() bool {
	return PermissionModel == V0
}

//  checks if the account permission allows the transaction to be executed
func IsTransactionAllowed(from common.Address, to common.Address, value *big.Int, gasPrice *big.Int, gasLimit *big.Int, payload []byte, transactionType TransactionType) error {
	//if we have not reached QIP714 block return full access
	if !PermissionsEnabled() {
		return nil
	}

	return PermissionTransactionAllowedFunc(from, to, value, gasPrice, gasLimit, payload, transactionType)
}
