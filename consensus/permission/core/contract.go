package v0

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/miniBamboo/luckyshare/consensus/permission/core"
	ptype "github.com/miniBamboo/luckyshare/consensus/permission/core/types"
	binding "github.com/miniBamboo/luckyshare/consensus/permission/v0/bind"
)

// definitions for v0 permissions model which is aligned with eea specs

type PermissionModelV0 struct {
	ContractBackend   ptype.ContractBackend
	PermInterf        *binding.PermInterface
	PermInterfSession *binding.PermInterfaceSession
}

type Control struct {
	Backend *PermissionModelV0
}

type Node struct {
	Backend *PermissionModelV0
}

type Init struct {
	Backend ptype.ContractBackend
	//binding contracts
	PermUpgr *binding.PermUpgr

	PermNode *binding.NodeManager

	//sessions
	permNodeSession *binding.NodeManagerSession
}

func (i *Init) UpdateNetworkBootStatus() (*types.Transaction, error) {
	return i.PermInterfSession.UpdateNetworkBootStatus()
}

func (i *Init) GetNodeDetailsFromIndex(_nodeIndex *big.Int) (string, string, *big.Int, error) {
	r, err := i.permNodeSession.GetNodeDetailsFromIndex(_nodeIndex)
	if err != nil {
		return "", "", big.NewInt(0), err
	}
	return r.OrgId, core.GetNodeUrl(r.EnodeId, r.Ip[:], r.Port, r.Authorityport, i.Backend.IsAuthority), r.NodeStatus, err
}

func (i *Init) GetNumberOfNodes() (*big.Int, error) {
	return i.permNodeSession.GetNumberOfNodes()
}

func (i *Init) GetNodeDetails(enodeId string) (string, string, *big.Int, error) {
	r, err := i.permNodeSession.GetNodeDetails(enodeId)
	if err != nil {
		return "", "", big.NewInt(0), err
	}
	return r.OrgId, core.GetNodeUrl(r.EnodeId, r.Ip[:], r.Port, r.Authorityport, i.Backend.IsAuthority), r.NodeStatus, err
}

func (i *Init) GetRoleDetails(_roleId string, _orgId string) (struct {
	RoleId     string
	OrgId      string
	AccessType *big.Int
	Voter      bool
	Admin      bool
	Active     bool
}, error) {
	return i.permRoleSession.GetRoleDetails(_roleId, _orgId)
}

func (i *Init) GetSubOrgIndexes(_orgId string) ([]*big.Int, error) {
	return i.permOrgSession.GetSubOrgIndexes(_orgId)
}

func (i *Init) GetOrgInfo(_orgIndex *big.Int) (string, string, string, *big.Int, *big.Int, error) {
	return i.permOrgSession.GetOrgInfo(_orgIndex)
}

func (i *Init) GetNetworkBootStatus() (bool, error) {
	return i.PermInterfSession.GetNetworkBootStatus()
}

func (i *Init) GetOrgDetails(_orgId string) (string, string, string, *big.Int, *big.Int, error) {
	return i.permOrgSession.GetOrgDetails(_orgId)
}

// This is to make sure all contract instances are ready and initialized
//
// Required to be call after standard service start lifecycle
func (i *Init) BindContracts() error {
	log.Debug("permission service: binding contracts")

	err := i.bindContract()
	if err != nil {
		return err
	}

	i.initSession()
	return nil
}

func (a *Audit) ValidatePendingOp(_authOrg, _orgId, _url string, _account common.Address, _pendingOp int64) bool {
	var enodeId string
	var err error
	if _url != "" {
		enodeId, _, _, _, err = getNodeDetails(_url, a.Backend.ContractBackend.IsAuthority, a.Backend.ContractBackend.UseDns)
		if err != nil {
			log.Error("permission: encountered error while checking for pending operations", "err", err)
			return false
		}
	}
	pOrg, pUrl, pAcct, op, err := a.Backend.PermInterfSession.GetPendingOp(_authOrg)
	return err == nil && (op.Int64() == _pendingOp && pOrg == _orgId && pUrl == enodeId && pAcct == _account)
}

func (a *Audit) CheckPendingOp(_orgId string) bool {
	_, _, _, op, err := a.Backend.PermInterfSession.GetPendingOp(_orgId)
	return err == nil && op.Int64() != 0
}

func (c *Control) ConnectionAllowed(_enodeId, _ip string, _port, _raftPort uint16) (bool, error) {
	url := core.GetNodeUrl(_enodeId, _ip, _port, _raftPort, c.Backend.ContractBackend.IsAuthority)
	enodeId, ip, port, _, err := getNodeDetails(url, c.Backend.ContractBackend.IsAuthority, c.Backend.ContractBackend.UseDns)
	if err != nil {
		return false, err
	}

	return c.Backend.PermInterfSession.ConnectionAllowed(enodeId, ip, port)
}

func (c *Control) TransactionAllowed(_sender common.Address, _target common.Address, _value *big.Int, _gasPrice *big.Int, _gasLimit *big.Int, _payload []byte, _transactionType core.TransactionType) error {
	if allowed, err := c.Backend.PermInterfSession.TransactionAllowed(_sender, _target, _value, _gasPrice, _gasLimit, _payload); err != nil {
		return err
	} else if !allowed {
		return ptype.ErrNoPermissionForTxn
	}
	return nil
}

func (r *Role) RemoveRole(_args ptype.TxArgs) (*types.Transaction, error) {
	return r.Backend.PermInterfSession.RemoveRole(_args.RoleId, _args.OrgId)
}

func (r *Role) AddNewRole(_args ptype.TxArgs) (*types.Transaction, error) {
	if _args.AccessType > 7 {
		return nil, fmt.Errorf("invalid access type given")
	}
	return r.Backend.PermInterfSession.AddNewRole(_args.RoleId, _args.OrgId, big.NewInt(int64(_args.AccessType)), _args.IsVoter, _args.IsAdmin)
}

func (o *Org) ApproveOrgStatus(_args ptype.TxArgs) (*types.Transaction, error) {
	return o.Backend.PermInterfSession.ApproveOrgStatus(_args.OrgId, big.NewInt(int64(_args.Action)))
}

func (o *Org) UpdateOrgStatus(_args ptype.TxArgs) (*types.Transaction, error) {
	return o.Backend.PermInterfSession.UpdateOrgStatus(_args.OrgId, big.NewInt(int64(_args.Action)))
}

func (o *Org) ApproveOrg(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, o.Backend.ContractBackend.IsAuthority, o.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}
	return o.Backend.PermInterfSession.ApproveOrg(_args.OrgId, enodeId, ip, port, raftPort, _args.AcctId)
}

func (o *Org) AddSubOrg(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, o.Backend.ContractBackend.IsAuthority, o.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}
	return o.Backend.PermInterfSession.AddSubOrg(_args.POrgId, _args.OrgId, enodeId, ip, port, raftPort)
}

func (o *Org) AddOrg(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, o.Backend.ContractBackend.IsAuthority, o.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}
	return o.Backend.PermInterfSession.AddOrg(_args.OrgId, enodeId, ip, port, raftPort, _args.AcctId)
}

func (n *Node) ApproveBlacklistedNodeRecovery(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, n.Backend.ContractBackend.IsAuthority, n.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}
	return n.Backend.PermInterfSession.ApproveBlacklistedNodeRecovery(_args.OrgId, enodeId, ip, port, raftPort)
}

func (n *Node) StartBlacklistedNodeRecovery(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, n.Backend.ContractBackend.IsAuthority, n.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}
	return n.Backend.PermInterfSession.StartBlacklistedNodeRecovery(_args.OrgId, enodeId, ip, port, raftPort)
}

func (n *Node) AddNode(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, n.Backend.ContractBackend.IsAuthority, n.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}

	return n.Backend.PermInterfSession.AddNode(_args.OrgId, enodeId, ip, port, raftPort)
}

func (n *Node) UpdateNodeStatus(_args ptype.TxArgs) (*types.Transaction, error) {
	enodeId, ip, port, raftPort, err := getNodeDetails(_args.Url, n.Backend.ContractBackend.IsAuthority, n.Backend.ContractBackend.UseDns)
	if err != nil {
		return nil, err
	}
	return n.Backend.PermInterfSession.UpdateNodeStatus(_args.OrgId, enodeId, ip, port, raftPort, big.NewInt(int64(_args.Action)))
}

func (i *Init) bindContract() error {
	if err := ptype.BindContract(&i.PermUpgr, func() (interface{}, error) {
		return binding.NewPermUpgr(i.Backend.PermConfig.UpgrdAddress, i.Backend.EthClnt)
	}); err != nil {
		return err
	}
	if err := ptype.BindContract(&i.PermInterf, func() (interface{}, error) {
		return binding.NewPermInterface(i.Backend.PermConfig.InterfAddress, i.Backend.EthClnt)
	}); err != nil {
		return err
	}
	if err := ptype.BindContract(&i.PermAcct, func() (interface{}, error) {
		return binding.NewAcctManager(i.Backend.PermConfig.AccountAddress, i.Backend.EthClnt)
	}); err != nil {
		return err
	}
	if err := ptype.BindContract(&i.PermNode, func() (interface{}, error) {
		return binding.NewNodeManager(i.Backend.PermConfig.NodeAddress, i.Backend.EthClnt)
	}); err != nil {
		return err
	}
	if err := ptype.BindContract(&i.PermRole, func() (interface{}, error) {
		return binding.NewRoleManager(i.Backend.PermConfig.RoleAddress, i.Backend.EthClnt)
	}); err != nil {
		return err
	}
	if err := ptype.BindContract(&i.PermOrg, func() (interface{}, error) {
		return binding.NewOrgManager(i.Backend.PermConfig.OrgAddress, i.Backend.EthClnt)
	}); err != nil {
		return err
	}
	return nil
}

func (i *Init) initSession() {
	auth := bind.NewKeyedTransactor(i.Backend.Key)
	log.Debug("NodeAccount V0", "nodeAcc", auth.From)
	i.PermInterfSession = &binding.PermInterfaceSession{
		Contract: i.PermInterf,
		CallOpts: bind.CallOpts{
			Pending: true,
		},
		TransactOpts: bind.TransactOpts{
			From:     auth.From,
			Signer:   auth.Signer,
			GasLimit: 47000000,
			GasPrice: big.NewInt(0),
		},
	}

	i.permOrgSession = &binding.OrgManagerSession{
		Contract: i.PermOrg,
		CallOpts: bind.CallOpts{
			Pending: true,
		},
	}

	i.permNodeSession = &binding.NodeManagerSession{
		Contract: i.PermNode,
		CallOpts: bind.CallOpts{
			Pending: true,
		},
	}

	//populate roles
	i.permRoleSession = &binding.RoleManagerSession{
		Contract: i.PermRole,
		CallOpts: bind.CallOpts{
			Pending: true,
		},
	}

	//populate accounts
	i.permAcctSession = &binding.AcctManagerSession{
		Contract: i.PermAcct,
		CallOpts: bind.CallOpts{
			Pending: true,
		},
	}
}

// checks if the passed URL is no nil and then calls GetNodeDetails
func getNodeDetails(url string, isAuthority, useDns bool) (string, string, uint16, uint16, error) {
	if len(url) > 0 {
		return ptype.GetNodeDetails(url, isAuthority, useDns)
	}

	return "", "", uint16(0), uint16(0), nil
}
