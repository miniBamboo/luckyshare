package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p/enode"
	//"github.com/ethereum/go-ethereum/params"
	//"github.com/ethereum/go-ethereum/raft"
)

// supports 2 models of permissions v1 and v0.
// v0 is aligned with the latest eea specs
const (
	PERMISSION_V0 = "v0"
	PERMISSION_V1 = "v1"
)
const (
	PERMISSIONED_CONFIG     = "permissioned-nodes.json"
	BLACKLIST_CONFIG        = "disallowed-nodes.json"
	PERMISSION_MODEL_CONFIG = "permission-config.json"
	DEFAULT_NODECACHE_SIZE  = 1000
	NODE_NAME_LENGTH        = 32
)

// permission config for bootstrapping
type PermissionConfig struct {
	PermissionsModel string         `json:"permissionModel"`
	UpgrdAddress     common.Address `json:"upgrdableAddress"`
	NodeAddress      common.Address `json:"nodeMgrAddress"`
	//Accounts         []common.Address `json:"accounts"` //initial list of account that need full access

}

var (
	ErrInvalidInput = errors.New("Invalid input")

	ErrInvalidNode = errors.New("Invalid enode id")

	ErrBlacklistedNode = errors.New("Blacklisted node. Operation not allowed")

	ErrNodeDoesNotExists = errors.New("Node does not exist")
	ErrOrgDoesNotExists  = errors.New("Org does not exist")

	ErrHostNameNotSupported = errors.New("Hostname not supported in the network")
	ErrNoPermissionForTxn   = errors.New("account does not have permission for the transaction")
)

// backend struct for interfaces
type InterfaceBackend struct {
	node        *node.Node
	isAuthority bool
	dataDir     string
}

func (i *InterfaceBackend) SetIsAuthority(isAuthority bool) {
	i.isAuthority = isAuthority
}

func NewInterfaceBackend(node *node.Node, isAuthority bool, dataDir string) *InterfaceBackend {
	return &InterfaceBackend{node: node, isAuthority: isAuthority, dataDir: dataDir}
}

func (i InterfaceBackend) Node() *node.Node {
	return i.node
}

func (i InterfaceBackend) IsAuthority() bool {
	return i.isAuthority
}

func (i InterfaceBackend) DataDir() string {
	return i.dataDir
}

// to signal all watches when service is stopped
type StopEvent struct {
}

// broadcasting stopEvent when service is being stopped
var StopFeed event.Feed
var mux sync.Mutex

type NodeOperation uint8

const (
	NodeAdd NodeOperation = iota
	NodeDelete
)

type Backend interface {

	// node service for node management service
	GetNodeService(transactOpts *bind.TransactOpts, nodeBackend ContractBackend) (NodeService, error)

	// control service for account management service
	GetControlService(controlBackend ContractBackend) (ControlService, error)

	// Monitors Node management events and updates cache accordingly
	ManageNodePermissions() error

	// monitors for network boot up complete event
	MonitorNetworkBootUp() error
}

// adds or deletes and entry from a given file
func UpdateFile(fileName, enodeId string, operation NodeOperation, createFile bool) error {
	// Load the nodes from the config file
	var nodeList []string
	index := 0
	// if createFile is false means the file is already existing. read the file
	if !createFile {
		blob, err := ioutil.ReadFile(fileName)
		if err != nil && !createFile {
			return err
		}

		if err := json.Unmarshal(blob, &nodeList); err != nil {
			return err
		}

		// logic to update the permissioned-nodes.json file based on action

		recExists := false
		for i, eid := range nodeList {
			if eid == enodeId {
				index = i
				recExists = true
				break
			}
		}
		if (operation == NodeAdd && recExists) || (operation == NodeDelete && !recExists) {
			return nil
		}
	}
	if operation == NodeAdd {
		nodeList = append(nodeList, enodeId)
	} else {
		nodeList = append(nodeList[:index], nodeList[index+1:]...)
	}
	blob, _ := json.Marshal(nodeList)

	mux.Lock()
	defer mux.Unlock()

	err := ioutil.WriteFile(fileName, blob, 0644)
	return err
}

//this function populates the black listed Node information into the disallowed-nodes.json file
func UpdateDisallowedNodes(dataDir, url string, operation NodeOperation) error {

	fileExists := true
	path := filepath.Join(dataDir, BLACKLIST_CONFIG)
	// Check if the file is existing. If the file is not existing create the file
	if _, err := os.Stat(path); err != nil {
		if _, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644); err != nil {
			return err
		}
		fileExists = false
	}

	if fileExists {
		err := UpdateFile(path, url, operation, false)
		return err
	} else {
		err := UpdateFile(path, url, operation, true)
		return err
	}
}

// Disconnect the Node from the network
func DisconnectNode(node *node.Node, enodeId string, isAuthority bool) error {
	if isAuthority { // TO DO?
		var raftService *raft.AuthorityService
		if err := node.Service(&raftService); err == nil {
			raftApi := raft.NewPublicAuthorityAPI(raftService)

			//get the raftId for the given enodeId
			raftId, err := raftApi.GetAuthorityId(enodeId)
			if err == nil {
				raftApi.RemovePeer(raftId)
			} else {
				return err
			}
		}
	} else {
		// Istanbul  or clique - disconnect the peer

		server := node.Server()
		if server != nil {
			node, err := enode.ParseV4(enodeId)
			if err == nil {
				server.RemovePeer(node)
			} else {
				return err
			}
		}
	}
	return nil
}

// updates Node information in the permissioned-nodes.json file based on Node
// management activities in smart contract
func UpdatePermissionedNodes(node *node.Node, dataDir, enodeId string, operation NodeOperation, isAuthority bool) error {
	path := filepath.Join(dataDir, PERMISSIONED_CONFIG)
	if _, err := os.Stat(path); err != nil {
		return err
	}

	err := UpdateFile(path, enodeId, operation, false)
	if err != nil {
		return err
	}
	if operation == NodeDelete {
		err := DisconnectNode(node, enodeId, isAuthority)
		if err != nil {
			return err
		}
	}
	return nil
}

// function to subscribe to the stop event
func SubscribeStopEvent() (chan StopEvent, event.Subscription) {
	c := make(chan StopEvent)
	s := StopFeed.Subscribe(c)
	return c, s
}

// function reads the permissions config file passed and populates the
// config structure accordingly
func ParsePermissionConfig(dir string) (PermissionConfig, error) {
	fullPath := filepath.Join(dir, PERMISSION_MODEL_CONFIG)
	f, err := os.Open(fullPath)
	if err != nil {
		log.Error("can't open file", "file", fullPath, "error", err)
		return PermissionConfig{}, err
	}
	defer func() {
		_ = f.Close()
	}()

	var permConfig PermissionConfig
	blob, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Error("error reading file", "err", err, "file", fullPath)
	}

	err = json.Unmarshal(blob, &permConfig)
	if err != nil {
		log.Error("error unmarshalling the file", "err", err, "file", fullPath)
	}

	if permConfig.PermissionsModel == "" {
		return PermissionConfig{}, fmt.Errorf("permissions model type not passed in %s. Network cannot boot up", PERMISSION_MODEL_CONFIG)
	}

	permConfig.PermissionsModel = strings.ToLower(permConfig.PermissionsModel)
	if permConfig.PermissionsModel != PERMISSION_V0 && permConfig.PermissionsModel != PERMISSION_V1 {
		return PermissionConfig{}, fmt.Errorf("invalid permissions model type passed in %s. Network cannot boot up", PERMISSION_MODEL_CONFIG)
	}

	if len(permConfig.Accounts) == 0 {
		return PermissionConfig{}, fmt.Errorf("no accounts given in %s. Network cannot boot up", PERMISSION_MODEL_CONFIG)
	}
	if permConfig.SubOrgDepth.Cmp(big.NewInt(0)) == 0 || permConfig.SubOrgBreadth.Cmp(big.NewInt(0)) == 0 {
		return PermissionConfig{}, fmt.Errorf("sub org breadth depth not passed in %s. Network cannot boot up", PERMISSION_MODEL_CONFIG)
	}
	if permConfig.IsEmpty() {
		return PermissionConfig{}, fmt.Errorf("missing contract addresses in %s", PERMISSION_MODEL_CONFIG)
	}

	return permConfig, nil
}

// returns the enode details
func GetNodeDetails(url string, isAuthority, useDns bool) (string, string, uint16, uint16, error) {
	// validate Node id and
	var ip string
	if len(url) == 0 {
		return "", ip, 0, 0, errors.New("invalid Node id. empty url")
	}
	enodeDet, err := enode.ParseV4(url)
	if err != nil {
		return "", ip, 0, 0, fmt.Errorf("invalid Node id. %s", err.Error())
	}

	ip = enodeDet.IP().String()
	if isAuthority && useDns {
		if enodeDet.Host() != "" {
			ip = enodeDet.Host()
		}
	}
	return enodeDet.EnodeID(), ip, uint16(enodeDet.TCP()), uint16(enodeDet.AuthorityPort()), err
}

func (pc *PermissionConfig) IsEmpty() bool {
	return pc.InterfAddress == common.HexToAddress("0x0")
}
