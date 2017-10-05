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

package softSwitch

import (
	"asicd/pluginManager/pluginCommon"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"utils/commonDefs"
	"utils/logging"

	"github.com/vishvananda/netlink"
)

type SoftSwitchPlugin struct {
	logger                  *logging.Writer
	controllingPlugin       bool
	linkStateCB             pluginCommon.ProcessLinkStateChangeCB
	portConfigCB            pluginCommon.InitPortConfigDBCB
	portStateInitCB         pluginCommon.InitPortStateDBCB
	portStateCB             pluginCommon.UpdatePortStateDBCB
	BufferPortStateInitCB   pluginCommon.InitBufferPortStateDBCB
	BufferPortStateCB       pluginCommon.UpdateBufferPortStateDBCB
	BufferGlobalStateInitCB pluginCommon.InitBufferGlobalStateDBCB
	BufferGlobalStateCB     pluginCommon.UpdateBufferGlobalStateDBCB
	//updateCoppStatStateCB   pluginCommon.UpdateCoppStatStateDBCB
	routeChannel chan *pluginCommon.PluginIPRouteInfo
}

type LinkIp struct {
	configured int
	lsIP       string
	gsIP       string
}

const (
	SOFTSWITCH_SYSCTL_WRITE = "/sbin/sysctl"
	// user will have to specifiy port like vlan100, fpPort12, vlan100.1
	SOFTSWITCH_IPV4_PORT_INFO         = "net/ipv4/conf/"
	SOFTSWITCH_IPV6_PORT_INFO         = "net/ipv6/conf/"
	SOFTSWITCH_IPV4_ALL_CONF_LOCATION = "/proc/sys/net/ipv4/conf/all/"
	SOFTSWITCH_SET_ARP_NOTIFY         = "/arp_notify"
	SOFTSWITCH_SET_ARP_FILTER         = "/arp_filter"
	SOFTSWITCH_SET_ARP_IGNORE         = "/arp_ignore"
	SOFTSWITCH_SET_ARP_ANNOUNCE       = "/arp_announce"
	SOFTSWITCH_IPV6_CONF_LOCATION     = "/proc/sys/net/ipv6/conf/"
	LINKSCAN_INTERVAL                 = 500 /*milliseconds*/
	SOFTSWITCH_LINK_OPERSTATE_INFO    = "/sys/class/net/"
	LINK_UP                           = "up"
	LINK_DOWN                         = "down"
	SS_NO_LINK_LOCAL_CONFIGURED       = "NOT_CONFIGURED"
)

var plugin SoftSwitchPlugin

func NewSoftSwitchPlugin(logger *logging.Writer,
	linkStateCB pluginCommon.ProcessLinkStateChangeCB,
	portConfigCB pluginCommon.InitPortConfigDBCB,
	portStateInitCB pluginCommon.InitPortStateDBCB,
	portStateCB pluginCommon.UpdatePortStateDBCB, BufferPortStateInitCB pluginCommon.InitBufferPortStateDBCB,
	BufferPortStateCB pluginCommon.UpdateBufferPortStateDBCB,
	BufferGlobalStateInitCB pluginCommon.InitBufferGlobalStateDBCB,
	BufferGlobalStateCB pluginCommon.UpdateBufferGlobalStateDBCB) *SoftSwitchPlugin {
	plugin.logger = logger
	plugin.linkStateCB = linkStateCB
	plugin.portConfigCB = portConfigCB
	plugin.portStateInitCB = portStateInitCB
	plugin.portStateCB = portStateCB
	plugin.BufferPortStateInitCB = BufferPortStateInitCB
	plugin.BufferPortStateCB = BufferPortStateCB
	plugin.BufferGlobalStateInitCB = BufferGlobalStateInitCB
	plugin.BufferGlobalStateCB = BufferGlobalStateCB
	plugin.routeChannel = make(chan *pluginCommon.PluginIPRouteInfo, 100000)
	return (&plugin)
}

//Contains a mapping of Asic front panel port to linux interface name
var linuxIntfNames map[int]string
var linuxIntfNamesSlice []int
var BlackHoleRoutes []*net.IPNet
var sviBase = pluginCommon.SVI_PREFIX
var linuxIpAddrInfo map[string]LinkIp // on an interface how many ipv6 address are configured

func disableIPv6PerPort(linkName string) {
	disableLocation := SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/disable_ipv6"
	if _, err := os.Stat(disableLocation); err == nil {
		err := ioutil.WriteFile(disableLocation, []byte("1"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to enable ipv6 on the link - ", linkName)
			return
		} else {
			plugin.logger.Debug("SS: disabled ipv6 for link -", linkName)
		}
	}
	ipv6EnableLoc := SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/ipv6_enable"
	if _, err := os.Stat(ipv6EnableLoc); err == nil {
		err = ioutil.WriteFile(ipv6EnableLoc, []byte("0"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to disable ipv6 on the link - ", linkName)
			return
		}
	}
}

func (p *SoftSwitchPlugin) disableIPv6() {
	for _, linkName := range linuxIntfNames {
		disableIPv6PerPort(linkName)
	}
}

func (p *SoftSwitchPlugin) setArpConf() {
	arpLoc := SOFTSWITCH_IPV4_ALL_CONF_LOCATION + SOFTSWITCH_SET_ARP_FILTER
	if _, err := os.Stat(arpLoc); err == nil {
		err = ioutil.WriteFile(arpLoc, []byte("1"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to set arp_filter to 1", arpLoc)
		}
	}
	arpLoc = SOFTSWITCH_IPV4_ALL_CONF_LOCATION + SOFTSWITCH_SET_ARP_IGNORE
	if _, err := os.Stat(arpLoc); err == nil {
		err = ioutil.WriteFile(arpLoc, []byte("2"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to set arp_ignore to 2", arpLoc)
		}
	}
	arpLoc = SOFTSWITCH_IPV4_ALL_CONF_LOCATION + SOFTSWITCH_SET_ARP_ANNOUNCE
	if _, err := os.Stat(arpLoc); err == nil {
		err = ioutil.WriteFile(arpLoc, []byte("2"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to set arp_announce to 2", arpLoc)
		}
	}
}

func getLinkState(ifName string) bool {
	var state bool
	var stateStr string
	_, err := netlink.LinkByName(ifName)
	if err == nil {
		fileName := SOFTSWITCH_LINK_OPERSTATE_INFO + ifName + "/" + "operstate"
		stateStrFile, err := ioutil.ReadFile(fileName)
		if err == nil {
			stateStr = strings.TrimSpace(string(stateStrFile))
		}
		if stateStr == LINK_UP {
			state = true
		} else {
			state = false
		}
	}
	return state
}

//Linkscan routine
func linkScanner() {
	var stateStr string
	var firstIter bool = true
	var linkState []bool = make([]bool, len(linuxIntfNames))
	//Set up intial condition
	for portNum, ifName := range linuxIntfNames {
		linkState[portNum] = getLinkState(ifName)
	}

	//Poll for state change
	ticker := time.NewTicker(LINKSCAN_INTERVAL * time.Millisecond)
	for _ = range ticker.C {
		for portNum, ifName := range linuxIntfNames {
			newState := getLinkState(ifName)
			if firstIter || (newState != linkState[portNum]) {
				//Update state
				linkState[portNum] = newState
				if newState {
					stateStr = pluginCommon.STATE_UP
				} else {
					stateStr = pluginCommon.STATE_DOWN
				}
				//Make link state change callback
				plugin.linkStateCB(int32(portNum), int32(1000), pluginCommon.FULL_DUPLEX, stateStr)
			}
		}
		firstIter = false
	}
}

//General plugin functions
func (p *SoftSwitchPlugin) Init(primaryPlugin bool, baseDir string, bootMode int, ifMap []pluginCommon.IfMapInfo) int {
	p.logger.Info("Softswitchplugin init")
	if VxlanDB == nil {
		VxlanDB = make(map[uint32]*VxlanDbEntry)
	}

	linuxIntfNames = make(map[int]string, pluginCommon.MAX_SYS_PORTS)
	linuxIpAddrInfo = make(map[string]LinkIp, pluginCommon.MAX_SYS_PORTS)
	BlackHoleRoutes = make([]*net.IPNet, 0)
	p.controllingPlugin = primaryPlugin
	if primaryPlugin {
		ifMapCount := len(ifMap)
		/* If count==1 and port==-1, then map all ports using interface name prefix specified */
		if (ifMapCount == 1) && (ifMap[0].Port == -1) {
			var portNum int = 0
			if primaryPlugin {
				links, err := netlink.LinkList()
				if err != nil {
					p.logger.Err("Softswitchplugin init:Failed to get list of links during linux plugin Init. Aborting !")
					return -1
				}
				linkNamePrefix := ifMap[0].IfName
				for _, link := range links {
					if strings.Contains(link.Attrs().Name, linkNamePrefix) {
						linuxIntfNames[portNum] = link.Attrs().Name
						// Used for plugin.InitPortStateDB
						linuxIntfNamesSlice = append(linuxIntfNamesSlice, portNum)
						portNum += 1
					}
				}
			}
		} else {
			/* Only map list of ports specified in config file to linux interfaes */
			for port, ifName := range ifMap {
				_, err := netlink.LinkByName(ifName.IfName)
				if err == nil {
					//Save link name if link is found
					linuxIntfNames[int(port)] = ifName.IfName
				}
			}
		}
		//Spawn a linkscan routine to handle link state change
		p.logger.Info("Softswitchplugin Init: Start link scanner")
		go linkScanner()
	}
	p.logger.Info("Softswitchplugin Init: Start route server")
	go p.StartRouteServer()
	return 0
}

func (p *SoftSwitchPlugin) StartRouteServer() {
	p.logger.Info("Softswitch Start Route Server go routine")
	for {
		select {
		case routeConf := <-p.routeChannel:
			p.logger.Debug("SoftSwitch routeServer received msg on routeConf channel with op:", routeConf.Op)
			if routeConf.Op == pluginCommon.PluginOp_Add {
				ret := p.CreateIPRoute(routeConf)
				p.logger.Debug("returning ", ret, " from softswitch route add")
				/*
					//routeConf.DoneChannel <- int(ret)
					if routeConf.RetHdlrFunc != nil {
						p.logger.Debug("rethdlr func not nil")
						routeConf.RetHdlrFunc(routeConf, routeConf.RouteManager, p, ret)
					} else {
						p.logger.Debug("rethdlr func nil")
					}
				*/
			} else if routeConf.Op == pluginCommon.PluginOp_Del {
				ret := p.DeleteIPRoute(routeConf)
				p.logger.Debug("returning ", ret, " from softswitch route del")
				//routeConf.DoneChannel <- int(ret)
				/*
					if routeConf.RetHdlrFunc != nil {
						p.logger.Debug("rethdlr func not nil")
						routeConf.RetHdlrFunc(routeConf, routeConf.RouteManager, p, ret)
					} else {
						p.logger.Debug("rethdlr func nil")
					}
				*/
			}
		}
	}
}

func (p *SoftSwitchPlugin) Deinit(saveState bool) int {
	p.logger.Info("SS Deinit")
	for _, dstIPNet := range BlackHoleRoutes {
		lxroute := netlink.Route{Dst: dstIPNet}
		err := netlink.RouteDel(&lxroute)
		if err != nil {
			p.logger.Err("Route delete call failed with error ", err, " for:", dstIPNet)
		}
	}
	return 0
}
func (p *SoftSwitchPlugin) DevShell() {
}

func getName(ifIndex int32) string {
	var ifName string = ""
	ifType := pluginCommon.GetTypeFromIfIndex(ifIndex)
	switch ifType {
	case commonDefs.IfTypePort:
		ifName = linuxIntfNames[int(ifIndex)]
	case commonDefs.IfTypeLag:
		ifName = pluginCommon.LAG_PREFIX + strconv.Itoa(pluginCommon.GetIdFromIfIndex(ifIndex))
	case commonDefs.IfTypeVtep:
		ifName = pluginCommon.VXLAN_VTEP_PREFIX + strconv.Itoa(pluginCommon.GetIdFromIfIndex(ifIndex))
	}
	return ifName
}

//Vlan related functions
func AddInterfacesToBridge(vlanId int, ifIndexList, untagIfIndexList []int32, bridgeName string, bridgeLink netlink.Link) int {
	var err error
	var link netlink.Link
	var linkAttrs netlink.LinkAttrs

	if len(ifIndexList) == 0 &&
		len(untagIfIndexList) == 0 {
		return -1
	}
	binary, lookErr := exec.LookPath("bridge")
	if lookErr != nil {
		plugin.logger.Err("SS: Error looking up bin util bridge during CreateVlan()")
		return -1
	}
	for _, ifIndex := range ifIndexList {
		ifName := getName(ifIndex)
		//create virtual vlan interface i.e. ifName.vlanId
		linkAttrs.Name = ifName + "." + strconv.Itoa(int(vlanId))
		parentLink, err := netlink.LinkByName(ifName)
		if err != nil {
			plugin.logger.Err("SS: Error retrieving link by name during CreateVlan()")
			return -1
		}
		parentLinkAttrs := parentLink.Attrs()
		linkAttrs.ParentIndex = parentLinkAttrs.Index
		link = &netlink.Vlan{linkAttrs, vlanId}
		err = netlink.LinkAdd(link)
		if err != nil {
			plugin.logger.Err("SS: LinkAdd failed when adding ports, during CreateVlan()")
			return -1
		}
		err = netlink.LinkSetUp(link)
		if err != nil {
			plugin.logger.Err("SS: LinkSetUp failed when adding ports, during CreateVlan()")
			return -1
		}
		//add the interface to the bridge
		err = netlink.LinkSetMaster(link, bridgeLink.(*netlink.Bridge))
		if err != nil {
			plugin.logger.Err("SS: Error setting link Master during CreateVlan()")
			return -1
		}
		intfName := ifName + "." + strconv.Itoa(int(vlanId))

		cmd := exec.Command(binary, "vlan", "add", "dev", intfName, "vid", strconv.Itoa(vlanId), "pvid", "untagged")
		err = cmd.Run()
		if err != nil {
			plugin.logger.Err("SS: Error executing 2nd bridge vlan command during CreateVlan()")
			return -1
		}

	}
	for _, ifIndex := range untagIfIndexList {
		ifName := getName(ifIndex)
		link, err = netlink.LinkByName(ifName)
		if err != nil {
			plugin.logger.Err("SS: Error retrieving link by name during CreateVlan()")
			return -1
		}
		if err = netlink.LinkSetMaster(link, bridgeLink.(*netlink.Bridge)); err != nil {
			plugin.logger.Err("SS: Error setting link Master during CreateVlan()")
			return -1
		}
		/*
			fileName := "/sys/class/net/" + bridgeName + "/bridge/vlan_filtering"
			if err = ioutil.WriteFile(fileName, []byte("1"), 0644); err != nil {
			    plugin.logger.Err("SS: Error writing to vlan_filtering sys file during CreateVlan()")
			    return -1
			}
		*/
		/*** temporary hack to call exec command****/
		cmd := exec.Command(binary, "vlan", "add", "dev", ifName, "vid", strconv.Itoa(vlanId), "pvid", "untagged")
		err = cmd.Run()
		if err != nil {
			plugin.logger.Err("SS: Error executing 2nd bridge vlan command during CreateVlan()")
			return -1
		}
	}
	cmd := exec.Command(binary, "vlan", "add", "dev", bridgeName, "vid", strconv.Itoa(vlanId), "self", "pvid", "untagged")
	err = cmd.Run()
	if err != nil {
		plugin.logger.Err("SS: Error executing 1st bridge vlan command during CreateVlan()")
		return -1
	}
	return 0
}
func DeleteInterfacesFromBridge(vlanId int, ifIndexList, untagIfIndexList []int32) int {
	var err error
	var link netlink.Link
	for _, ifIndex := range ifIndexList {
		ifName := getName(ifIndex)
		//delete virtual vlan interface
		linkName := ifName + "." + strconv.Itoa(vlanId)
		link, err = netlink.LinkByName(linkName)
		if err != nil {
			plugin.logger.Err("SS: Error retrieving link by name during DeleteVlan()")
			return -1
		}
		if err = netlink.LinkDel(link); err != nil {
			plugin.logger.Err("SS: Error deleting vlan link during DeleteVlan()")
			return -1
		}
	}
	for _, ifIndex := range untagIfIndexList {
		ifName := getName(ifIndex)
		link, err = netlink.LinkByName(ifName)
		if err != nil {
			plugin.logger.Err("SS: Error retrieving link by name during DeleteVlan()")
			return -1
		}
		//remove the interface from the bridge
		err = netlink.LinkSetMasterByIndex(link, 0)
		if err != nil {
			plugin.logger.Err("SS: Error setting link Master during DeleteVlan()")
			return -1
		}
	}
	return 0
}
func (p *SoftSwitchPlugin) CreateVlan(rsvd bool, vlanId int, ifIndexList, untagIfIndexList []int32) int {
	if rsvd {
		p.logger.Info("Reserved vlan, do nothing")
		return 0
	}
	var linkAttrs netlink.LinkAttrs
	//create bridgelink - SVI<vlan>
	bridgeName := sviBase + strconv.Itoa(vlanId)
	bridgeLink, err := netlink.LinkByName(bridgeName)
	if err != nil {
		linkAttrs.Name = bridgeName
		bridgeLink = &netlink.Bridge{linkAttrs}
		err = netlink.LinkAdd(bridgeLink)
		if err != nil {
			p.logger.Err("SS: LinkAdd call failed during CreateVlan() ")
			return -1
		}
		AddInterfacesToBridge(vlanId, ifIndexList, untagIfIndexList, bridgeName, bridgeLink)
		err = netlink.LinkSetUp(bridgeLink)
		if err != nil {
			p.logger.Err("SS: LinkSetUp call failed during CreateVlan()")
			return -1
		}
		disableIPv6PerPort(bridgeName)
	}
	return 0
}
func (p *SoftSwitchPlugin) DeleteVlan(rsvd bool, vlanId int, ifIndexList, untagIfIndexList []int32) int {
	if rsvd {
		p.logger.Info("Reserved vlan, do nothing")
		return 0
	}
	bridgeName := sviBase + strconv.Itoa(vlanId)
	bridgeLink, err := netlink.LinkByName(bridgeName)
	if err != nil {
		p.logger.Err("SS: Could not locate bridge during DeleteVlan() - rsvd, vlanId", rsvd, vlanId)
		return -1
	}
	DeleteInterfacesFromBridge(vlanId, ifIndexList, untagIfIndexList)
	if err = netlink.LinkDel(bridgeLink); err != nil {
		p.logger.Err("SS: Error deleting bridge link during DeleteVlan() - rsvd, vlanId", rsvd, vlanId)
		return -1
	}
	return 0
}

func (p *SoftSwitchPlugin) UpdateVlan(vlanId int, oldIfIndexList, oldUntagIfIndexList, newIfIndexList, newUntagIfIndexList []int32) int {
	//create bridgelink - SVI<vlan>
	bridgeName := sviBase + strconv.Itoa(vlanId)
	bridgeLink, _ := netlink.LinkByName(bridgeName)
	deleteIfIndexList := pluginCommon.ComputeSetDifference(oldIfIndexList, newIfIndexList)
	deleteUntagIfIndexList := pluginCommon.ComputeSetDifference(oldUntagIfIndexList, newUntagIfIndexList)
	DeleteInterfacesFromBridge(vlanId, deleteIfIndexList, deleteUntagIfIndexList)
	addIfIndexList := pluginCommon.ComputeSetDifference(newIfIndexList, oldIfIndexList)
	addUntagIfIndexList := pluginCommon.ComputeSetDifference(newUntagIfIndexList, oldUntagIfIndexList)
	AddInterfacesToBridge(vlanId, addIfIndexList, addUntagIfIndexList, bridgeName, bridgeLink)
	return 0
}

//Port related functions
func (p *SoftSwitchPlugin) GetMaxSysPorts() int {
	numintfs := 0
	for _, ifname := range linuxIntfNames {
		_, err := netlink.LinkByName(ifname)
		if err == nil {
			// interface exists
			numintfs++
		}
	}
	return numintfs
}

func (p *SoftSwitchPlugin) DeleteLinuxIPV6LSAddress(link netlink.Link) {
	entry, exists := linuxIpAddrInfo[link.Attrs().Name]
	if !exists {
		// no ipv6 create happend
		return
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		plugin.logger.Err("SS: Failed to get ipv6 address list during port up for", link.Attrs().Name)
		return
	}
	for _, addr := range addrs {
		if addr.IP.IsLinkLocalUnicast() {
			if strings.Compare(entry.lsIP, addr.IPNet.String()) != 0 {
				plugin.logger.Debug("Calling deleteAutoConfiguredLinkScopeIp for addr:", addr.IPNet.String())
				//deleting auto-configured ipv6 link scope ip address
				deleteAutoConfiguredLinkScopeIp(link, addr.IPNet.String())
			}
		}
	}
}

func (p *SoftSwitchPlugin) AddIPv6AddressBack(link netlink.Link) {
	linkName := link.Attrs().Name
	ipInfo, exists := linuxIpAddrInfo[linkName]
	if !exists {
		return
	}

	if ipInfo.lsIP != "" {
		err := AddIPAddrToNetLink(ipInfo.lsIP, link)
		if err != nil {
			p.logger.Err("SS: Assinging ip during link UP:", ipInfo.lsIP, "to port:", linkName, "failed:", err)
		}
	}
	if ipInfo.gsIP != "" {
		err := AddIPAddrToNetLink(ipInfo.gsIP, link)
		if err != nil {
			p.logger.Err("SS: Assinging ip during link UP:", ipInfo.gsIP, "to port:", linkName, "failed:", err)
		}
	}
}

func (p *SoftSwitchPlugin) UpdatePortConfig(flags int32, newPortObj *pluginCommon.PortConfig, breakOutPortsList []int32) int {
	for _, portNum := range breakOutPortsList {
		linkName := linuxIntfNames[int(portNum)]
		disableIPv6PerPort(linkName)
	}
	if flags&pluginCommon.PORT_ATTR_ADMIN_STATE == pluginCommon.PORT_ATTR_ADMIN_STATE {
		netif, err := netlink.LinkByName(newPortObj.PortName)
		if err == nil {
			if newPortObj.AdminState == pluginCommon.STATE_UP {
				err := netlink.LinkSetUp(netif)
				if err != nil {
					p.logger.Err("Failed to Admin UP interface - ", newPortObj.PortName)
				}
				// if ipv6 intf create happens before link up then we might end up having
				// two link scope ip addresses.
				// in that case we need to delete auto-configured link scope ip address
				// @TODO: once we move away from linux for ipv6 then remove this
				p.DeleteLinuxIPV6LSAddress(netif)
				// fixing: WD-140 IPv6 address removed in linux when link is shutdown
				p.AddIPv6AddressBack(netif)
			} else {
				err := netlink.LinkSetDown(netif)
				if err != nil {
					p.logger.Err("Failed to Admin DOWN interface - ", newPortObj.PortName)
				}
			}
		} else {
			p.logger.Err("Failed to obtain netlink handle for interface - ", newPortObj.PortName)
		}
	}
	return 0
}
func (p *SoftSwitchPlugin) InitPortConfigDB() int {
	var portConfig pluginCommon.PortConfig
	for portNum, ifname := range linuxIntfNames {
		netif, err := netlink.LinkByName(ifname)
		if err == nil {
			// interface exists
			netifattr := netif.Attrs()
			portConfig.PortNum = int32(portNum)
			portConfig.Description = "Host ethernet Port"
			portConfig.PhyIntfType = netif.Type()
			portConfig.AdminState = pluginCommon.UpDownState[1]
			portConfig.MacAddr = netifattr.HardwareAddr.String()
			// TODO to gather speed / autoneg use ethtool
			// https://github.com/intelsdi-x/snap-plugin-collector-ethtool/blob/master/ethtool/collector.go
			portConfig.Speed = 1000 // 1G ???
			portConfig.Duplex = "FULL"
			portConfig.Autoneg = "AUTO"
			portConfig.MediaType = netif.Type()
			portConfig.Mtu = int32(netifattr.MTU)
			plugin.portConfigCB(&portConfig)
		}
	}
	return 0
}

func retrieveIfStatsByIfName(ifname string, portState *pluginCommon.PortState) {
	var statsValueList []int64 = make([]int64, 8)
	statFilePathSuffix := "/statistics/"
	statFilePathPrefix := "/sys/class/net/"
	statsTypeList := []string{"rx_bytes", "rx_packets", "rx_dropped", "rx_errors", "tx_bytes", "tx_packets", "tx_dropped", "tx_errors"}

	for idx := 0; idx < len(statsTypeList); idx++ {
		fileName := statFilePathPrefix + ifname + "/" + statFilePathSuffix + statsTypeList[idx]
		statStrFile, err := ioutil.ReadFile(fileName)
		if err != nil {
			continue
		}
		statStr := strings.TrimSpace(string(statStrFile))
		statVal, err := strconv.Atoi(statStr)
		if err != nil {
			continue
		}
		statsValueList[idx] = int64(statVal)
	}
	portState.IfInOctets = statsValueList[0]
	portState.IfInUcastPkts = statsValueList[1]
	portState.IfInDiscards = statsValueList[2]
	portState.IfInErrors = statsValueList[3]
	//Missing linux equivalent
	portState.IfInUnknownProtos = 0
	portState.IfOutOctets = statsValueList[4]
	portState.IfOutUcastPkts = statsValueList[5]
	portState.IfOutDiscards = statsValueList[6]
	portState.IfOutErrors = statsValueList[7]
}

func (p *SoftSwitchPlugin) InitPortStateDB(portMap map[int32]string) int {
	if p.controllingPlugin {
		for _, portNum := range linuxIntfNamesSlice {
			ifname := linuxIntfNames[portNum]
			//link, err := netlink.LinkByName(ifname)
			_, err := netlink.LinkByName(ifname)
			if err == nil {
				// interface exists
				plugin.portStateInitCB(int32(portNum), ifname)
			}
		}
	} else {
		for portNum, ifName := range portMap {
			linuxIntfNames[int(portNum)] = ifName
		}
	}
	// during init we should disable all ipv6 creations
	p.disableIPv6()

	// set arp filter, ignore and announce
	p.setArpConf()
	return 0
}

func (p *SoftSwitchPlugin) UpdatePortStateDB(startPort, endPort int32) int {
	var portState pluginCommon.PortState
	for portNum, ifname := range linuxIntfNames {
		if portNum >= int(startPort) && portNum <= int(endPort) {
			netif, err := netlink.LinkByName(ifname)
			if err == nil {
				// interface exists
				netifattr := netif.Attrs()
				portState.PortNum = int32(portNum)
				portState.IfIndex = pluginCommon.GetIfIndexFromIdType(int(portNum), commonDefs.IfTypePort)
				portState.Name = netifattr.Name
				if getLinkState(ifname) {
					portState.OperState = pluginCommon.STATE_UP
				} else {
					portState.OperState = pluginCommon.STATE_DOWN
				}
				retrieveIfStatsByIfName(ifname, &portState)
				plugin.portStateCB(&portState)
			}
		}
	}
	return 0
}

func (p *SoftSwitchPlugin) InitBufferPortStateDB(portMap map[int32]string) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateBufferPortStateDB(startPort, endPort int32) int {
	return 0
}

func (p *SoftSwitchPlugin) InitBufferGlobalStateDB(deviceId int) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateBufferGlobalStateDB(startId, endId int) int {
	return 0
}

//Lag related functions
func (p *SoftSwitchPlugin) RestoreLagDB() int {
	return 0
}
func (p *SoftSwitchPlugin) CreateLag(obj *pluginCommon.PluginLagInfo) int {
	bondIfName := obj.IfName
	var linkAttrs = netlink.LinkAttrs{
		Name:         bondIfName,
		HardwareAddr: pluginCommon.SwitchMacAddr,
	}

	bondedIf := netlink.NewLinkBond(linkAttrs)
	bondedIf.Mode = netlink.BOND_MODE_ACTIVE_BACKUP
	if obj.HashType == pluginCommon.HASHTYPE_SRCMAC_DSTMAC {
		bondedIf.XmitHashPolicy = netlink.BondXmitHashPolicy(0)
	} else if obj.HashType == pluginCommon.HASHTYPE_SRCIP_DSTIP {
		bondedIf.XmitHashPolicy = netlink.BondXmitHashPolicy(1)
	}
	bondedIf.MinLinks = 1
	err := netlink.LinkAdd(bondedIf)
	if err != nil {
		p.logger.Err("SS: CreateLag err from Bond LinkAdd = ", err)
		return -1
	}
	if bondedIf, err := netlink.LinkByName(bondIfName); err == nil {
		p.AddPortToBondIf(bondedIf, obj.MemberList)
		err = netlink.LinkSetUp(bondedIf)
		if err != nil {
			p.logger.Err("SS: LinkSetUp call failed during CreateLag()")
			return -1
		}
	}
	return 0
}

func (p *SoftSwitchPlugin) DeleteLag(obj *pluginCommon.PluginLagInfo) int {
	bondIfName := obj.IfName
	if bondedif, err := netlink.LinkByName(bondIfName); err == nil {
		netlink.LinkDel(bondedif)
	}
	return 0
}

// algorithm assumes lists are not sorted
func GetListDifference(l1, l2 []int32) (diffList []int32) {
	for _, op := range l1 {
		foundPort := false
		for _, np := range l2 {
			if op == np {
				foundPort = true
				break
			}
		}
		if foundPort == false {
			diffList = append(diffList, op)
		}
	}
	return diffList
}

func (p *SoftSwitchPlugin) AddPortToBondIf(bondedIf netlink.Link, portList []int32) int {
	// lets add the new ports
	for _, port := range portList {
		ifName, ok := linuxIntfNames[int(port)]
		linkif, err := netlink.LinkByName(ifName)
		if ok && err == nil {
			// link should be down before we add it to the bonded interface
			err = netlink.LinkSetDown(linkif)
			if err != nil {
				p.logger.Err("SS: Add Port err from Link LinkSetDown = ", err)
			}

			linkif.Attrs().ParentIndex = bondedIf.Attrs().Index
			err = netlink.LinkSetMasterByIndex(linkif, bondedIf.Attrs().Index)
			if err != nil {
				p.logger.Err("SS: Add Port err from Add Link to Bond LinkSetMasterByIndex = ", err)
			}

			err = netlink.LinkSetUp(linkif)
			if err != nil {
				p.logger.Err("SS: Add Port err from LinkSetUp = ", err)
			}
			//p.logger.Err("SS: Adding interface", linkname, "to bonded interface", bondname)
		} else {
			p.logger.Err("SS: Add Port Unable to find", port)
			return -1
		}
	}

	return 0
}

func (p *SoftSwitchPlugin) DelPortFromBondIf(bondedif netlink.Link, portList []int32) int {
	for _, port := range portList {
		ifName, ok := linuxIntfNames[int(port)]
		linkif, err := netlink.LinkByName(ifName)
		if ok && err == nil {
			// link should be down before we add it to the bonded interface
			err = netlink.LinkSetDown(linkif)
			if err != nil {
				p.logger.Err("SS: Del Port err from LinkSetDown = ", err)
			}

			linkif.Attrs().ParentIndex = bondedif.Attrs().Index
			err = netlink.LinkSetNoMaster(linkif)
			if err != nil {
				p.logger.Err("SS: Del Port err from LinkSetNoMaster = ", err)
			}
			err = netlink.LinkSetUp(linkif)
			if err != nil {
				p.logger.Err("SS: Del Port err from LinkSetUp = ", err)
			}

		} else {
			p.logger.Err("SS: Del Port Unable to find", port)
			return -1
		}
		//fmt.Println("Deleting interface", linkname, "from bonded interface", bondname)
	}
	return 0
}

func (p *SoftSwitchPlugin) UpdateLag(obj *pluginCommon.PluginUpdateLagInfo) int {
	delPortList := GetListDifference(obj.OldMemberList, obj.MemberList)
	addPortList := GetListDifference(obj.MemberList, obj.OldMemberList)

	bondIfName := obj.IfName
	if bondedif, err := netlink.LinkByName(bondIfName); err == nil {
		// TODO
		//if hashType == pluginCommon.HASHTYPE_SRCMAC_DSTMAC {
		//	bondedif.XmitHashPolicy = netlink.BondXmitHashPolicy(0)
		//} else if hashType == pluginCommon.HASHTYPE_SRCIP_DSTIP {
		//	bondedif.XmitHashPolicy = netlink.BondXmitHashPolicy(1)
		//}
		//nl.NewRtAttrChild(data, nl.IFLA_BOND_XMIT_HASH_POLICY, nl.Uint8Attr(uint8(bondedIf.XmitHashPolicy)))
		p.DelPortFromBondIf(bondedif, delPortList)
		p.AddPortToBondIf(bondedif, addPortList)
	} else {
		p.logger.Err("SS: UpdateLag Unable to find", bondIfName)
		return -1
	}
	return 0
}

//IPv4 Neighbor related functions
func (p *SoftSwitchPlugin) RestoreIPNeighborDB() int {
	return 0
}

func (p *SoftSwitchPlugin) CreateIPNeighbor(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return 0
}

func (p *SoftSwitchPlugin) DeleteIPNeighbor(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateIPNeighbor(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return 0
}

//IP route related functions
//IP Route related functions
func (p *SoftSwitchPlugin) AddToRouteChannel(ipInfo *pluginCommon.PluginIPRouteInfo) {
	//p.logger.Debug("DoftSwitchPlugin AddToRouteChannel")
	p.routeChannel <- ipInfo
}
func (p *SoftSwitchPlugin) CreateIPRoute(ipInfo *pluginCommon.PluginIPRouteInfo) int {
	p.logger.Debug("CreateIpRoute in softswitch")
	var err error
	lxroute := netlink.Route{}
	if pluginCommon.IsZeroIP(ipInfo.NextHopIp, ipInfo.NextHopIpType) {
		return 0
	}
	var prefixIp []uint8
	if ipInfo.IpType == syscall.AF_INET {
		prefixIp = ((net.IP(ipInfo.PrefixIp)).Mask(net.IPMask((net.IP(ipInfo.Mask)).To4())))
		//p.logger.Info("ipv4:ipInfo.PrefixIp:", ipInfo.PrefixIp, " ipInfp.Mask:", ipInfo.Mask, " prefixIp:", prefixIp)
	} else {
		prefixIp = ((net.IP(ipInfo.PrefixIp)).Mask(net.IPMask((net.IP(ipInfo.Mask)).To16())))
		//p.logger.Info("ipv6:ipInfo.PrefixIp:", ipInfo.PrefixIp, " ipInfp.Mask:", ipInfo.Mask, " prefixIp:", prefixIp)
	}
	//p.logger.Info("ipInfo.PrefixIp:", ipInfo.PrefixIp, " ipInfp.Mask:", ipInfo.Mask, " prefixIp:", prefixIp)
	dstIPNet := &net.IPNet{
		IP:   net.IP(prefixIp),
		Mask: net.IPMask(ipInfo.Mask),
	}
	if (ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_NULL) == pluginCommon.ROUTE_TYPE_NULL {
		p.logger.Info("blackhole route in softswitch createiproute")
		lxroute = netlink.Route{Dst: dstIPNet, Type: syscall.RTN_BLACKHOLE}
		BlackHoleRoutes = append(BlackHoleRoutes, dstIPNet)
	} else {
		link, err := netlink.LinkByName(ipInfo.IfName)
		if err != nil {
			p.logger.Err("SS:LinkByName call failed with error ", err, "for linkName ", ipInfo.IfName)
			return -1
		}
		lxroute = netlink.Route{LinkIndex: link.Attrs().Index, Dst: dstIPNet, Gw: net.IP(ipInfo.NextHopIp)}
	}
	if ipInfo.RouteFlags&pluginCommon.ROUTE_TYPE_MULTIPATH == pluginCommon.ROUTE_TYPE_MULTIPATH {
		//p.logger.Info("ecmp route")
		err = netlink.RouteAppend(&lxroute)
	} else {
		//p.logger.Info("non ecmp route")
		err = netlink.RouteAdd(&lxroute)
	}
	if err != nil {
		p.logger.Err("SS: Route add call failed with error ", err)
		return -1
	}
	return 0
}

//IPv4 Route related functions
func (p *SoftSwitchPlugin) RestoreIPv4RouteDB() int {
	return 0
}
func (p *SoftSwitchPlugin) DeleteIPRoute(ipInfo *pluginCommon.PluginIPRouteInfo) int {
	if pluginCommon.IsZeroIP(ipInfo.NextHopIp, ipInfo.NextHopIpType) {
		p.logger.Info("Connected route, do nothing")
		return 0
	}
	var prefixIp []uint8
	lxroute := netlink.Route{}
	if ipInfo.IpType == syscall.AF_INET {
		prefixIp = ((net.IP(ipInfo.PrefixIp)).Mask(net.IPMask((net.IP(ipInfo.Mask)).To4())))
	} else {
		prefixIp = ((net.IP(ipInfo.PrefixIp)).Mask(net.IPMask((net.IP(ipInfo.Mask)).To16())))
	}
	dstIPNet := &net.IPNet{
		IP:   net.IP(prefixIp),
		Mask: net.IPMask(ipInfo.Mask),
	}
	if (ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_NULL) == pluginCommon.ROUTE_TYPE_NULL {
		p.logger.Info("blackhole route in softswitch deleteiproute")
		lxroute = netlink.Route{Dst: dstIPNet, Type: syscall.RTN_BLACKHOLE}
	} else {
		lxroute = netlink.Route{Dst: dstIPNet, Gw: net.IP(ipInfo.NextHopIp)}
	}
	err := netlink.RouteDel(&lxroute)
	if err != nil {
		p.logger.Err("Route delete call failed with error ", err)
		return -1
	}
	return 0
}

//IPv6 Route related functions
func (p *SoftSwitchPlugin) RestoreIPv6RouteDB() int {
	return 0
}

//NextHop functions
func (p *SoftSwitchPlugin) RestoreIPv4NextHopDB() {
}

func (p *SoftSwitchPlugin) CreateIPNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) uint64 {
	return uint64(0)
}

func (p *SoftSwitchPlugin) DeleteIPNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateIPNextHop(nbrInfo *pluginCommon.PluginIPNeighborInfo) int {
	return 0
}

//NextHop Group functions
func (p *SoftSwitchPlugin) RestoreIPv4NextHopGroupDB() {
}

func (p *SoftSwitchPlugin) CreateIPNextHopGroup(nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo) uint64 {
	return uint64(0)
}

func (p *SoftSwitchPlugin) DeleteIPNextHopGroup(groupId uint64) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateIPNextHopGroup(groupId uint64, nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo) int {
	return 0
}

//Logical Interface related functions
func (p *SoftSwitchPlugin) CreateLogicalIntfConfig(name string, ifType int) int {
	p.logger.Info("Softswitch CreateLogicalIntfConfig")
	var linkAttrs netlink.LinkAttrs
	//create loopbacki/f
	liLink, err := netlink.LinkByName(name)
	if err != nil {
		linkAttrs.Name = name
		linkAttrs.Flags = syscall.IFF_LOOPBACK
		linkAttrs.HardwareAddr = pluginCommon.SwitchMacAddr
		liLink = &netlink.Dummy{linkAttrs} //,"loopback"}
		err = netlink.LinkAdd(liLink)
		if err != nil {
			p.logger.Err("SS: LinkAdd call failed during CreateLogicalIntfConfig() ", err)
			return -1
		}
		err = netlink.LinkSetUp(liLink)
		if err != nil {
			p.logger.Err("SS: LinkSetUp call failed during CreateVlan()")
			return -1
		}
	}
	return 0
}
func (p *SoftSwitchPlugin) DeleteLogicalIntfConfig(name string, ifType int) int {
	link, err := netlink.LinkByName(name)
	if link == nil {
		p.logger.Err("SS: Failed to find logical if link during DeleteLogicalIntfConfig()", name)
		return -1
	}
	err = netlink.LinkDel(link)
	if err != nil {
		p.logger.Err("SS: Error while deleting logical intf link ", name, " during DeleteLogicalIntfConfig")
		return -1
	}
	return 0
}

/* API: Helper API to add ip address to netlink
 */
func AddIPv4AddrToNetlink(ipAddr uint32, maskLen int, link netlink.Link) error {
	var ip net.IP
	var mask uint32 = 0xFF000000
	for idx := 0; idx < 4; idx++ {
		b := ipAddr & mask
		ip = append(ip, byte(b>>((3-uint(idx))*8)))
		mask >>= 8
	}
	addr, err := netlink.ParseAddr(ip.String() + "/" + strconv.Itoa(maskLen))
	if err != nil {
		plugin.logger.Err("SS: Error while parsing ip address during CreateIPIntf()")
		return err
	}
	plugin.logger.Info("Configuring ", addr, "for", link.Attrs().Name)
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		plugin.logger.Err("SS: Error while assigning ip",
			"address to bridge during CreateIPIntf() : ", err)
		return err
	}
	return nil
}

func AddIPAddrToNetLink(ipAddr string, link netlink.Link) error {
	addr, err := netlink.ParseAddr(ipAddr)
	if err != nil {
		plugin.logger.Err("SS: Error while parsing ip address during CreateIPIntf()")
		return err
	}
	plugin.logger.Info("SS: Configuring", addr, "for", link.Attrs().Name)
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		plugin.logger.Err("SS: Error while assigning ip address to bridge during CreateIPIntf() : ", err)
		return err
	}
	return nil
}

func getLink(ifIndex int32, ifName string) (string, netlink.Link) {
	var linkName string
	l2Ref := pluginCommon.GetIdFromIfIndex(int32(ifIndex))
	l2RefType := pluginCommon.GetTypeFromIfIndex(int32(ifIndex))

	//set the ip interface on bridge<vlan>
	if l2RefType == commonDefs.IfTypeVlan {
		linkName = sviBase + strconv.Itoa(l2Ref)
	} else if l2RefType == commonDefs.IfTypePort || l2RefType == commonDefs.IfTypeLag {
		linkName = linuxIntfNames[l2Ref]
	} else {
		linkName = ifName
	}
	link, _ := netlink.LinkByName(linkName)
	return linkName, link
}

func getLinkLocalIp(link netlink.Link) (linklocalIP string) {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		plugin.logger.Err("SS: Failed to get ipv6 address list for", link.Attrs().Name)
		return SS_NO_LINK_LOCAL_CONFIGURED
	}
	for _, addr := range addrs {
		if addr.IP.IsLinkLocalUnicast() {
			return addr.IPNet.String()
		}
	}
	return SS_NO_LINK_LOCAL_CONFIGURED
}

func enableRAForForwardingLinuxIntf(linkName string) {
	raLocation := SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/accept_ra"
	if _, err := os.Stat(raLocation); err == nil {
		err := ioutil.WriteFile(raLocation, []byte("2"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to set accept_ra even if forwarding is enabled for link",
				linkName)
		}
	}
}

func enableIPv6PerPortLinuxIntf(linkName string) {
	disableLocation := SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/disable_ipv6"
	if _, err := os.Stat(disableLocation); err == nil {
		err := ioutil.WriteFile(disableLocation, []byte("0"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to enable ipv6 on the link - ", linkName)
		}
	}
	ipv6EnableLoc := SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/ipv6_enable"
	if _, err := os.Stat(ipv6EnableLoc); err == nil {
		err = ioutil.WriteFile(ipv6EnableLoc, []byte("1"), os.FileMode(644))
		if err != nil {
			plugin.logger.Err("SS: Failed to enable ipv6 on the link - ", linkName)
		}
	}
}

func deleteAutoConfiguredLinkScopeIp(link netlink.Link, ipAddr string) {
	addr, err := netlink.ParseAddr(ipAddr)
	if err != nil {
		plugin.logger.Err("SS: Error while parsing ip address:", ipAddr, "during deleteAutoConfiguredLinkScopeIp")
		return //-1
	}
	plugin.logger.Info("Deleting auto conf link local ip:", ipAddr, "for", link.Attrs().Name)
	err = netlink.AddrDel(link, addr)
	if err != nil {
		plugin.logger.Err("SS: Error while deleting ip address from link", link.Attrs().Name,
			"during deleteAutoConfiguredLinkScopeIp")
	}
}

//IP interface related functions
func (p *SoftSwitchPlugin) CreateIPIntf(ipInfo *pluginCommon.PluginIPInfo) int {
	p.logger.Debug("SS: CreateIPIntf for ipInfo", *ipInfo)
	var err error
	linkName := ipInfo.IfName
	// Get Link
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		p.logger.Err("SS: Failed to find link during CreateIPIntf()", linkName)
		return -1
	}
	// Check Ip type and construct proc sys string
	var sysProcStr string
	switch ipInfo.IpType {
	case syscall.AF_INET:
		sysProcStr = "/proc/sys/net/ipv4/conf/" + linkName + "/forwarding"
	case syscall.AF_INET6:
		// For IPV6 we need to enable ipv6 first
		// @HACK: once we decide to add systcl then this needs to go away...
		// Some linux doesn't have this ipv6_enable file in proc directory so first check whether
		// a file exists or not
		enableIPv6PerPortLinuxIntf(linkName)
		sysProcStr = SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/forwarding"
	}
	// If Link Local Ip then get existing link local ip if any and create a new one
	if ipInfo.IPv6Type == 2 { //{ @HACK: Nasty one fix this jgheewala change 2 if LINK_LOCAL_IP is changed
		ipInfo.LinkLocalIp = getLinkLocalIp(link)
		// if link local ip then delete the entry first and then add the entry
		// Proceed with linux deletion
		// if link local ip is same as ipInfo Address then no need to delete and add ip address
		p.logger.Debug("link auto-configured LS ip by linux is:", ipInfo.LinkLocalIp)
		if strings.Compare(ipInfo.LinkLocalIp, ipInfo.Address) != 0 {
			if ipInfo.LinkLocalIp != SS_NO_LINK_LOCAL_CONFIGURED {
				deleteAutoConfiguredLinkScopeIp(link, ipInfo.LinkLocalIp)
			}
		}
	}

	err = AddIPAddrToNetLink(ipInfo.Address, link)
	if err != nil {
		p.logger.Err("SS: Assinging ip:", ipInfo.Address, "to port:", linkName, "failed:", err)
		return -1
	}
	//Enable ip forwarding for this link
	err = ioutil.WriteFile(sysProcStr, []byte("1"), os.FileMode(644))
	if err != nil {
		plugin.logger.Err("SS: Failed to enable ip forwarding on the link - ", linkName)
		return -1
	}

	// After ip address is assigned and if its v6 type IP Address then enable v6 specific stuff
	if syscall.AF_INET6 == ipInfo.IpType {
		enableRAForForwardingLinuxIntf(linkName)
		entry, exists := linuxIpAddrInfo[linkName]
		if exists {
			entry.configured++
		} else {
			entry.configured = 1
		}
		if ipInfo.IPv6Type == 2 {
			entry.lsIP = ipInfo.Address
		} else {
			entry.gsIP = ipInfo.Address
		}
		p.logger.Debug("SS: ipv6 address cached on", linkName, "is:", entry)
		linuxIpAddrInfo[linkName] = entry
	}

	return 0
}

func (p *SoftSwitchPlugin) UpdateIPIntf(ipInfo *pluginCommon.PluginIPInfo) int {
	return 0
}

func (p *SoftSwitchPlugin) DeleteIPIntf(ipInfo *pluginCommon.PluginIPInfo) int {
	var err error
	linkName := ipInfo.IfName
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		p.logger.Err("SS: Failed to find bridge link during DeleteIPIntf()", linkName)
		return -1
	}
	// Before disabling or deleting ip address get link local ip for that link
	if ipInfo.IpType == syscall.AF_INET6 {
		ipInfo.LinkLocalIp = getLinkLocalIp(link)
	}
	// Proceed with linux deletion
	addr, err := netlink.ParseAddr(ipInfo.Address)
	if err != nil {
		plugin.logger.Err("SS: Error while parsing ip address during DeleteIPIntf()")
		return -1
	}
	plugin.logger.Info("Deleting ", addr, "for", link.Attrs().Name)
	err = netlink.AddrDel(link, addr)
	if err != nil {
		p.logger.Err("SS: Error while deleting ip address from bridge during DeleteIPv4Intf()")
		return -1
	}

	var sysProcStr string
	switch ipInfo.IpType {
	case syscall.AF_INET:
		sysProcStr = "/proc/sys/net/ipv4/conf/" + linkName + "/forwarding"
	case syscall.AF_INET6:
		entry, exists := linuxIpAddrInfo[linkName]
		if exists {
			entry.configured--
			if entry.configured == 0 {
				entry.lsIP = ""
				entry.gsIP = ""
			}
		}
		linuxIpAddrInfo[linkName] = entry
		if entry.configured == 0 {
			p.logger.Debug("SS:No More IPv6 address left on", linkName, "hence disabling ipv6")
			disableIPv6PerPort(linkName)
			sysProcStr = SOFTSWITCH_IPV6_CONF_LOCATION + linkName + "/forwarding"
		} else {
			p.logger.Debug("SS:There are", entry, " ipv6 address still configured on", linkName,
				"hence not disabling ipv6")
			return 0
		}
	}
	//Disable ip forwarding for this link
	err = ioutil.WriteFile(sysProcStr, []byte("0"), os.FileMode(644))
	if err != nil {
		plugin.logger.Err("Failed to disable ip forwarding on the link, file - ", linkName, sysProcStr)
		return -1
	}
	return 0
}

/* API: To configure mac addr:
* Steps: Bring link DOWN
*	  Configure MacAddr
*	  Bring link UP
 */
func AddSubIntfMacAddr(sublink netlink.Link, macAddr net.HardwareAddr) (err error) {
	// Set Mac Addr
	if macAddr != nil {
		err := netlink.LinkSetDown(sublink)
		if err != nil {
			plugin.logger.Err("link down failed", err)
		}
		plugin.logger.Info("Setting Mac addr to " + macAddr.String())
		err = netlink.LinkSetHardwareAddr(sublink, macAddr)
		if err != nil {
			plugin.logger.Err("SS: Failed to set link harwareAddr",
				macAddr.String(), "during create sub ipv4 intf, Error:",
				err)
			return err
		}
		err = netlink.LinkSetUp(sublink)
		if err != nil {
			plugin.logger.Err("link up failed", err)
		}
	}
	return nil
}

/* API: To configure ip addr:
* Steps: Bring link UP
*	  Configure Ip Addr
 */
func AddSubIntfIpAddr(sublink netlink.Link, ipAddr uint32, maskLen int) (err error) {
	err = AddIPv4AddrToNetlink(ipAddr, maskLen, sublink)
	if err != nil {
		plugin.logger.Err(err)
		return err
	}
	return nil
}

/*  API: to check given name whether a link exists or not
 */
func LinkExist(linkName string) bool {
	newlink, _ := netlink.LinkByName(linkName)
	if newlink != nil {
		return true
	}
	return false
}

//Sub IPv4 interface related functions
func (p *SoftSwitchPlugin) CreateSubIPv4Intf(obj pluginCommon.SubIntfPluginObj,
	ifName *string) int {
	var err error
	var linkName string
	var sublinkName string
	var sublink netlink.Link
	var parentLinkType string
	var sublinkAttrs netlink.LinkAttrs
	var parentLinkAttrs *netlink.LinkAttrs

	l2Ref := pluginCommon.GetIdFromIfIndex(obj.IfIndex)
	l2RefType := pluginCommon.GetTypeFromIfIndex(obj.IfIndex)
	//set the ip interface on bridge<vlan>
	if l2RefType == commonDefs.IfTypeVlan {
		linkName = sviBase + strconv.Itoa(l2Ref)
	} else if l2RefType == commonDefs.IfTypePort ||
		l2RefType == commonDefs.IfTypeLag {
		linkName = linuxIntfNames[l2Ref]
	} else {
		linkName = "dummy"
	}

	// Get parent link first
	parentLink, _ := netlink.LinkByName(linkName)
	if parentLink == nil {
		p.logger.Err("SS: Failed to find link during CreateSubIPv4Intf()", linkName)
		goto early_exit
	}
	// Set link name
	sublinkName = linkName + "." + strconv.Itoa(pluginCommon.GetIdFromIfIndex(obj.SubIntfIfIndex))
	// Create a secondary/virtual link on top of parent link
	parentLinkType = parentLink.Type()
	parentLinkAttrs = parentLink.Attrs()

	// if sub interface is secondary then we use parent type
	if pluginCommon.GetTypeFromIfIndex(obj.SubIntfIfIndex) == commonDefs.IfTypeSecondary {
		switch parentLinkType {
		case "dummy":
			sublink = &netlink.Dummy{}
		case "ifb":
			sublink = &netlink.Ifb{}
		case "bridge":
			sublink = &netlink.Bridge{}
		case "vlan":
			sublink = &netlink.Vlan{}
		case "veth":
			sublink = &netlink.Veth{}
		case "vxlan":
			sublink = &netlink.Vxlan{}
		case "bond":
			sublink = &netlink.Bond{}
		case "ipvlan":
			sublink = &netlink.IPVlan{}
		case "macvlan":
			sublink = &netlink.Macvlan{}
		case "macvtap":
			sublink = &netlink.Macvtap{}
		case "gretap":
			sublink = &netlink.Gretap{}
		default:
			sublink = &netlink.GenericLink{LinkType: parentLinkType}
		}
		// copy parentlink Attribute
		sublinkAttrs = netlink.LinkAttrs{
			Name:  sublinkName,
			MTU:   parentLinkAttrs.MTU,
			Flags: parentLinkAttrs.Flags,
		}
		*sublink.Attrs() = sublinkAttrs
	} else {
		sublink = &netlink.Macvlan{
			LinkAttrs: netlink.LinkAttrs{
				Name:        sublinkName,
				ParentIndex: parentLinkAttrs.Index,
			},
			Mode: netlink.MACVLAN_MODE_PRIVATE,
		}
	}
	// First add the link with default attributes like index,
	// flags and parent index
	err = netlink.LinkAdd(sublink)
	if err != nil {
		p.logger.Err("SS: failed to add link, Error:", err)
		goto early_exit
	}

	// Add Mac Addr...only if ifType
	if pluginCommon.GetTypeFromIfIndex(obj.SubIntfIfIndex) == commonDefs.IfTypeVirtual {
		err = AddSubIntfMacAddr(sublink, obj.MacAddr)
		if err != nil {
			p.logger.Err("SS: failed to add HardwareAddr, Error:", err)
			goto early_exit
		}
	}

	// Add Ip Addr
	err = AddSubIntfIpAddr(sublink, obj.IpAddr, obj.MaskLen)
	if err != nil {
		p.logger.Err("Adding Ip address failed, Error:", err)
		goto early_exit
	}

	// Always set link at the end so that we can retain user requested config
	// Set link up/down depending upon the stateUp flag passed by user
	if obj.StateUp {
		err = netlink.LinkSetUp(sublink)
	} else {
		err = netlink.LinkSetDown(sublink)
	}
	if err != nil {
		p.logger.Err("Changing link state failed, Error:", err)
		goto early_exit
	}

early_exit:
	if LinkExist(sublinkName) && err != nil {
		// remove the link
		err = netlink.LinkDel(sublink)
		if err != nil {
			p.logger.Err("deleting netlink on error failed", err)
		}
		return -1
	} else if err != nil { // corner case when link add itself failed :D
		return -1
	}
	*ifName = sublinkName
	return 0
}

func (p *SoftSwitchPlugin) DeleteSubIPv4Intf(ifName string) int {
	link, _ := netlink.LinkByName(ifName)
	if link != nil {
		if err := netlink.LinkDel(link); err != nil {
			p.logger.Err("deleting netlink failed", err)
			return -1
		}
	} else {
		p.logger.Err("No link found for " + ifName)
	}
	return 1
}

//@FIXME: when golang has support for sysctl then remove this..
func UpdateArpSysControl(ifName string, value int) int {
	cmd := SOFTSWITCH_SYSCTL_WRITE
	configArg := SOFTSWITCH_IPV4_PORT_INFO + ifName + SOFTSWITCH_SET_ARP_NOTIFY + "=" + strconv.Itoa(value)
	args := []string{"-w", configArg}
	if err := exec.Command(cmd, args...).Run(); err != nil {
		plugin.logger.Err("SS: Error executing sysctl command ERROR:", err)
		return -1
	}
	return 0
}

func (p *SoftSwitchPlugin) UpdateSubIPv4Intf(ifName string, mac net.HardwareAddr, ipAddr uint32, stateUp bool) int {
	link, err := netlink.LinkByName(ifName)
	if link == nil {
		p.logger.Err("cannot Update config, error:", err)
		return -1
	}
	err = AddSubIntfMacAddr(link, mac)
	if err != nil {
		p.logger.Err("SS: failed to add HardwareAddr, Error:", err)
		return -1
	}
	if stateUp {
		err = netlink.LinkSetUp(link)
		UpdateArpSysControl(ifName, 1)
	} else {
		err = netlink.LinkSetDown(link)
		UpdateArpSysControl(ifName, 0)
	}
	if err != nil {
		p.logger.Err("SS: changing link state failed, ERROR:", err)
		return -1
	}

	return 1
}

func (p *SoftSwitchPlugin) PortEnableSet(portNum int32, adminState string) int {
	return 0
}

//STP related functions
func (p *SoftSwitchPlugin) CreateStg(vlanList []int32) int {
	return 0
}

func (p *SoftSwitchPlugin) DeleteStg(stgId int32) int {
	return 0
}

func (p *SoftSwitchPlugin) SetPortStpState(stgId, port, stpState int32) int {
	return 0
}

func (p *SoftSwitchPlugin) GetPortStpState(stgId, port int32) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateStgVlanList(stgId int32, oldVlanList, newVlanList []int32) int {
	return 0
}

func (p *SoftSwitchPlugin) FlushFdbStgGroup(vlanList []int32, port int32) {
}

func (p *SoftSwitchPlugin) AddProtocolMacEntry(macAddr string, mask string,
	VlanId int32) int {
	return 1
}

func (p *SoftSwitchPlugin) DeleteProtocolMacEntry(macAddr string, mask string,
	VlanId int32) int {
	return 1
}

func (p *SoftSwitchPlugin) PortProtocolEnable(port, protocol int32, ena bool) {
}

func (v *SoftSwitchPlugin) CreateAclConfig(aclName string, aclType string, aclRule pluginCommon.AclRule, portList []int32, direction string) int {
	return 1
}

func (v *SoftSwitchPlugin) DeleteAcl(aclName string, direction string) int {
	return 1
}

func (v *SoftSwitchPlugin) UpdateAclRule(aclName string, acl pluginCommon.AclRule) int {
	return 1
}

func (v *SoftSwitchPlugin) DeleteAclRuleFromAcl(aclName string, acl pluginCommon.AclRule, intfList []int32, direction string) int {
	return 1
}

func (p *SoftSwitchPlugin) ClearPortStat(portNum int32) int {
	return 0
}

func (p *SoftSwitchPlugin) UpdateCoppStatStateDB(startId, endId int) int {
	return 0
}
func (p *SoftSwitchPlugin) GetModuleTemperature() float64 {
	//TO-DO
	return 0
}

func (p *SoftSwitchPlugin) GetModuleInventory() *pluginCommon.Inventory {
	//TO-DO
	return nil
}
func (p *SoftSwitchPlugin) UpdateAclStateDB(ruleName string) int {
	return 0
}

func (p *SoftSwitchPlugin) CreateIPIntfLoopback(*pluginCommon.PluginIPInfo) int {
	return 0
}
func (p *SoftSwitchPlugin) DeleteIPIntfLoopback(*pluginCommon.PluginIPInfo) int {
	return 0
}