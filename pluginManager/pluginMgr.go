//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//

package pluginManager

import (
	"asicd/pluginManager/bcmsdk"
	"asicd/pluginManager/opennsl"
	"asicd/pluginManager/pluginCommon"
	"asicd/pluginManager/sai"
	"asicd/pluginManager/softSwitch"
	"asicd/publisher"
	"asicdInt"
	"fmt"
	"net"
	"utils/dbutils"
	"utils/eventUtils"
	"utils/logging"
)

//This interfaces lists the APIs that all plugins are required to implement
type PluginIntf interface {
	//General plugin mgmt functions
	Init(primaryPlugin bool, baseDir string, bootMode int, ifMap []pluginCommon.IfMapInfo) int
	Deinit(saveState bool) int
	DevShell()
	//Global functions
	GetModuleInventory() *pluginCommon.Inventory
	GetModuleTemperature() float64
	//Vlan functions
	CreateVlan(rsvd bool, vlanId int, portList, untagPortList []int32) int
	DeleteVlan(rsvd bool, vlanId int, portList, untagPortList []int32) int
	UpdateVlan(vlanId int, oldPortList, oldUntagPortList, newPortList, newUntagPortList []int32) int
	//Port functions
	GetMaxSysPorts() int
	UpdatePortConfig(updateFlags int32, newPortObj *pluginCommon.PortConfig, bl []int32) int
	InitPortConfigDB() int
	InitPortStateDB(map[int32]string) int
	UpdatePortStateDB(startPort, endPort int32) int
	InitBufferPortStateDB(map[int32]string) int
	UpdateBufferPortStateDB(startPort, endPort int32) int
	//Lag functions
	RestoreLagDB() int
	CreateLag(obj *pluginCommon.PluginLagInfo) int
	DeleteLag(obj *pluginCommon.PluginLagInfo) int
	UpdateLag(obj *pluginCommon.PluginUpdateLagInfo) int
	//STP functions
	CreateStg(vlanList []int32) int
	DeleteStg(stgId int32) int
	SetPortStpState(stgId, port, stpState int32) int
	GetPortStpState(stgId, port int32) int
	UpdateStgVlanList(stgId int32, oldVlanList, newVlanList []int32) int
	FlushFdbStgGroup(vlanList []int32, port int32)
	//IPv4 Neighbor functions
	RestoreIPNeighborDB() int
	CreateIPNeighbor(*pluginCommon.PluginIPNeighborInfo) int
	DeleteIPNeighbor(*pluginCommon.PluginIPNeighborInfo) int
	UpdateIPNeighbor(*pluginCommon.PluginIPNeighborInfo) int
	//NextHop functions
	RestoreIPv4NextHopDB()
	CreateIPNextHop(*pluginCommon.PluginIPNeighborInfo) uint64
	DeleteIPNextHop(*pluginCommon.PluginIPNeighborInfo) int
	UpdateIPNextHop(*pluginCommon.PluginIPNeighborInfo) int
	//NextHop Group functions
	RestoreIPv4NextHopGroupDB()
	CreateIPNextHopGroup(nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo) uint64
	DeleteIPNextHopGroup(groupId uint64) int
	UpdateIPNextHopGroup(groupId uint64, nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo) int
	//Logical Intf functions
	CreateLogicalIntfConfig(name string, ifType int) int
	DeleteLogicalIntfConfig(name string, ifType int) int
	//IPv4 interface functions
	CreateIPIntf(*pluginCommon.PluginIPInfo) int
	UpdateIPIntf(*pluginCommon.PluginIPInfo) int
	DeleteIPIntf(*pluginCommon.PluginIPInfo) int

	//IP Route functions
	AddToRouteChannel(ipInfo *pluginCommon.PluginIPRouteInfo)
	RestoreIPv4RouteDB() int
	RestoreIPv6RouteDB() int
	CreateIPRoute(ipInfo *pluginCommon.PluginIPRouteInfo) int //, routeFlags uint32, nextHopId uint64, rIfId int32, ifName string) int
	DeleteIPRoute(ipInfo *pluginCommon.PluginIPRouteInfo) int //, routeFlags uint32) int

	//Protocol Reserved Mac Create/Delete Functions handled by L2 Manager
	AddProtocolMacEntry(macAddr string, mask string, VlanId int32) int
	DeleteProtocolMacEntry(macAddr string, mask string, VlandId int32) int
	// Err-Disable
	PortEnableSet(portNum int32, adminState string) int
	// Sub IPv4 interface
	CreateSubIPv4Intf(obj pluginCommon.SubIntfPluginObj, ifName *string) int
	DeleteSubIPv4Intf(ifName string) int
	UpdateSubIPv4Intf(ifName string, macAddr net.HardwareAddr, ipAddr uint32, stateUp bool) int
	// vxlan
	CreateVxlan(config *asicdInt.Vxlan)
	DeleteVxlan(config *asicdInt.Vxlan)
	AddInterfaceToVxlanBridge(vni uint32, ifIndexList, untagIfIndexList []int32)
	DelInterfaceFromVxlanBridge(vni uint32, ifIndexList, untagIfIndexList []int32)
	// ifname is src interface,
	// nextHopIfIndex should be the id returned from CreateIPv4Neighbor
	CreateVtepIntf(vtepIfName string, ifIndex int, nextHopIfIndex int, vni int32, dstIpAddr uint32, srcIpAddr uint32, macAddr net.HardwareAddr, VlanId int32, ttl int32, udp int32, nextHopIndex uint64)
	DeleteVtepIntf(vtepIfName string, ifIndex int, nextHopIfIndex int, vni int32, dstIpAddr uint32, srcIpAddr uint32, macAddr net.HardwareAddr, VlanId int32, ttl int32, udp int32)
	LearnFdbVtep(mac net.HardwareAddr, name string, ifIndex int32)
	VxlanPortEnable(portNum int32)
	VxlanPortDisable(portNum int32)
	//bst stat
	InitBufferGlobalStateDB(deviceId int) int
	UpdateBufferGlobalStateDB(startId, endId int) int
	//Acl
	CreateAclConfig(aclName string, aclType string, acl pluginCommon.AclRule, intfList []int32, direction string) int
	UpdateAclStateDB(ruleName string) int
	//UpdateAcl()int
	//DeleteAcl()int

	// Port Stats
	ClearPortStat(port int32) int
	UpdateCoppStatStateDB(startId, endId int) int
}

type InitParams struct {
	DbHdl              *dbutils.DBUtil
	BaseDir            string
	BootMode           int
	SysRsvdVlanMin     int
	SysRsvdVlanMax     int
	IfMap              []pluginCommon.IfMapInfo
	SwitchMac          string
	MacFlapDisablePort bool
	EnablePM           bool
}

/*
 * This is the top level plugin manager that :
 * - Contains a list of all instantiated plugins
 * - Encapsulates and manages all data common to plugins
 */

type ResourceManagers struct {
	*VlanManager
	*PortManager
	*LagManager
	*NextHopManager
	*NeighborManager
	*RouteManager
	*L3IntfManager
	/*
		     * VR : Temporarily commenting out mpls code from asicd
			 *	   	*MplsIntfManager
	*/
	*L2Manager
	*StpManager
	*TunnelManager
	*IfManager
	*AclManager
	*BstStatManager
	*InfraManager
	*PerfManager
}

type PluginManager struct {
	*ResourceManagers
	logger        *logging.Writer
	plugins       []PluginIntf
	notifyChannel *publisher.PubChannels
}

var controllingPlugin string

func NewPluginMgr(pluginList []string, logger *logging.Writer, notifyChannel *publisher.PubChannels) *PluginManager {
	pluginMgr := new(PluginManager)
	pluginMgr.ResourceManagers = new(ResourceManagers)
	pluginMgr.plugins = make([]PluginIntf, 0)
	for _, plugin := range pluginList {
		if plugin == "opennsl" {
			//Instantiate opennsl plugin
			opennslPlugin := opennsl.NewOpenNslPlugin(
				logger,
				notifyChannel.All,
				ProcessLinkStateChange,
				InitPortConfigDB,
				InitPortStateDB,
				UpdatePortStateDB,
				UpdateLagDB,
				UpdateIPNeighborDB,
				UpdateIPv4RouteDB,
				UpdateIPv4NextHopDB,
				UpdateIPv4NextHopGroupDB,
				UpdateMacDB,
				InitBufferPortStateDB,
				UpdateBufferPortStateDB,
				InitBufferGlobalStateDB,
				UpdateBufferGlobalStateDB,
				UpdateCoppStatStateDB,
				UpdateAclStateDB,
			)
			pluginMgr.plugins = append(pluginMgr.plugins, opennslPlugin)
		} else if plugin == "sai" {
			//Instantiate sai plugin

			saiPlugin := sai.NewSaiPlugin(
				logger,
				notifyChannel.All,
				ProcessLinkStateChange,
				InitPortConfigDB,
				InitPortStateDB,
				UpdatePortStateDB,
				UpdateLagDB,
				UpdateIPNeighborDB,
				UpdateIPv4RouteDB,
				UpdateIPv4NextHopDB,
				UpdateIPv4NextHopGroupDB,
				UpdateMacDB,
				InitBufferPortStateDB,
				UpdateBufferPortStateDB,
				InitBufferGlobalStateDB,
				UpdateBufferGlobalStateDB,
				//UpdateAclStateDB,
			//	UpdateCoppStatStateDB,
			)
			pluginMgr.plugins = append(pluginMgr.plugins, saiPlugin)
		} else if plugin == "linux" {
			//Instantiate linux plugin
			softSwitchPlugin := softSwitch.NewSoftSwitchPlugin(logger,
				ProcessLinkStateChange,
				InitPortConfigDB,
				InitPortStateDB,
				UpdatePortStateDB,
				InitBufferPortStateDB,
				UpdateBufferPortStateDB,
				InitBufferGlobalStateDB,
				UpdateBufferGlobalStateDB,
			//	UpdateAclStateDB,
			//	UpdateCoppStatStateDB,
			)
			pluginMgr.plugins = append(pluginMgr.plugins, softSwitchPlugin)
		} else if plugin == "bcmsdk" {
			// Instantiate bcmsdk plugin
			bcmsdkPlugin := bcmsdk.NewBcmSdkPlugin(
				logger,
				notifyChannel.All,
				ProcessLinkStateChange,
				InitPortConfigDB,
				InitPortStateDB,
				UpdatePortStateDB,
				UpdateLagDB,
				UpdateIPNeighborDB,
				UpdateIPv4RouteDB,
				UpdateIPv4NextHopDB,
				UpdateIPv4NextHopGroupDB,
				UpdateMacDB,
				InitBufferPortStateDB,
				UpdateBufferPortStateDB,
				InitBufferGlobalStateDB,
				UpdateBufferGlobalStateDB,
				UpdateCoppStatStateDB,
				//UpdateAclStateDB,
			)
			pluginMgr.plugins = append(pluginMgr.plugins, bcmsdkPlugin)
		}
	}
	controllingPlugin = pluginList[0]
	pluginMgr.VlanManager = &VlanMgr
	pluginMgr.PortManager = &PortMgr
	pluginMgr.LagManager = &LagMgr
	pluginMgr.NextHopManager = &NextHopMgr
	pluginMgr.NeighborManager = &NeighborMgr
	pluginMgr.RouteManager = &RouteMgr
	pluginMgr.L3IntfManager = &L3IntfMgr
	/* VR : Temporarily commenting out MPLS code in asicd
	pluginMgr.MplsIntfManager = &MplsIntfMgr
	*/
	pluginMgr.L2Manager = &L2Mgr
	pluginMgr.StpManager = &StpMgr
	pluginMgr.TunnelManager = &TunnelfMgr
	pluginMgr.AclManager = &AclMgr
	pluginMgr.IfManager = &IfMgr
	pluginMgr.BstStatManager = &BstStatMgr
	pluginMgr.InfraManager = &InfraMgr
	pluginMgr.PerfManager = &PerfMgr
	pluginMgr.logger = logger
	pluginMgr.notifyChannel = notifyChannel
	return pluginMgr
}

func (pMgr *PluginManager) Init(params *InitParams) {
	var err error
	pluginCommon.SwitchMacAddr, err = net.ParseMAC(params.SwitchMac)
	if err != nil {
		pMgr.logger.Err("Failed to parse switch mac address, using default mac")
		pluginCommon.SwitchMacAddr, err = net.ParseMAC(pluginCommon.DEFAULT_SWITCH_MAC_ADDR)
	}

	//Initialize Events
	err = eventUtils.InitEvents("ASICD", params.DbHdl, params.DbHdl, pMgr.logger, 1000)
	if err != nil {
		pMgr.logger.Err(fmt.Sprintln("Unable to initialize events", err))
	}
	//Initialize ifmanager first
	pMgr.IfManager.Init(pMgr.logger)
	//Initialize all plugins
	for _, plugin := range pMgr.plugins {
		rv := plugin.Init(isControllingPlugin(plugin), params.BaseDir, params.BootMode, params.IfMap)
		if rv < 0 {
			pMgr.logger.Err("Plugin initialization failed")
		}
	}
	//Initialize all resource managers
	pMgr.PortManager.Init(pMgr.ResourceManagers, params.DbHdl, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	pMgr.LagManager.Init(pMgr.ResourceManagers, params.BootMode, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	pMgr.VlanManager.Init(pMgr.ResourceManagers, params.DbHdl, params.BootMode, params.SysRsvdVlanMin, params.SysRsvdVlanMax, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	pMgr.NextHopManager.Init(pMgr.ResourceManagers, params.BootMode, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	pMgr.NeighborManager.Init(pMgr.ResourceManagers, params.BootMode, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	pMgr.RouteManager.Init(pMgr.ResourceManagers, params.BootMode, pMgr.plugins, pMgr.logger, pMgr.notifyChannel)
	pMgr.L3IntfManager.Init(pMgr.ResourceManagers, params.DbHdl, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	/* VR : Temporarily commenting out MPLS code in asicd
	pMgr.MplsIntfManager.Init(pMgr.ResourceManagers, params.DbHdl, pMgr.plugins, pMgr.logger, pMgr.notifyChannel)
	*/
	pMgr.L2Manager.Init(pMgr.ResourceManagers, params.DbHdl, pMgr.plugins, pMgr.logger, params.MacFlapDisablePort)
	pMgr.StpManager.Init(pMgr.ResourceManagers, pMgr.plugins, pMgr.logger)
	pMgr.TunnelManager.Init(pMgr.ResourceManagers, pMgr.plugins, pMgr.logger, pMgr.notifyChannel.All)
	pMgr.AclManager.Init(pMgr.ResourceManagers, params.DbHdl, pMgr.plugins, pMgr.logger)
	pMgr.BstStatManager.Init(pMgr.ResourceManagers, pMgr.plugins, pMgr.logger)
	pMgr.InfraManager.Init(pMgr.ResourceManagers, pMgr.plugins, pMgr.logger)
	pMgr.PerfManager.Init(pMgr.ResourceManagers, pMgr.plugins, pMgr.logger, params.EnablePM)
}

func (pMgr *PluginManager) Deinit(saveState bool) {
	//Deinit all resource managers first
	pMgr.L3IntfManager.Deinit()
	/* VR : Temporarily commenting out MPLS code in asicd
	 *
	pMgr.MplsIntfManager.Deinit()
	*/
	pMgr.PerfManager.Deinit()
	pMgr.InfraManager.Deinit()
	pMgr.BstStatManager.Deinit()
	pMgr.VlanManager.Deinit()
	pMgr.LagManager.Deinit()
	pMgr.PortManager.Deinit()
	pMgr.NeighborManager.Deinit()
	pMgr.RouteManager.Deinit()
	pMgr.L2Manager.Deinit()
	pMgr.StpManager.Deinit()
	pMgr.TunnelManager.Deinit()
	pMgr.IfManager.Deinit()
	//Deinit all plugins
	for _, plugin := range pMgr.plugins {
		rv := plugin.Deinit(saveState)
		if rv < 0 {
			pMgr.logger.Err("Failed to deinit plugins")
		}
	}
}

func isControllingPlugin(plugin PluginIntf) bool {
	switch plugin.(type) {
	case *sai.SaiPlugin:
		if controllingPlugin == "sai" {
			return true
		} else {
			return false
		}
	case *opennsl.OpenNslPlugin:
		if controllingPlugin == "opennsl" {
			return true
		} else {
			return false
		}
	case *bcmsdk.BcmSdkPlugin:
		if controllingPlugin == "bcmsdk" {
			return true
		} else {
			return false
		}
	case *softSwitch.SoftSwitchPlugin:
		if controllingPlugin == "linux" {
			return true
		} else {
			return false
		}
	default:
		return false
	}
}

func isPluginAsicDriver(plugin PluginIntf) bool {
	switch plugin.(type) {
	case *sai.SaiPlugin, *opennsl.OpenNslPlugin, *bcmsdk.BcmSdkPlugin:
		return true
	default:
		return false
	}
}

/*
 * The following objects do not have a resource manager. Their corresponding
 * methods are directly relayed by the plugin manager
 */
func (pMgr *PluginManager) DevShell() {
	for _, plugin := range pMgr.plugins {
		if isPluginAsicDriver(plugin) == true {
			plugin.DevShell()
		}
	}
}
