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
	_ "asicd/asicdCommonDefs"
	"asicd/pluginManager/pluginCommon"
	"asicd/publisher"
	"asicdInt"
	"asicdServices"
	_ "encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
	"utils/commonDefs"
	"utils/logging"
	netutils "utils/netUtils"
)

const (
	INITIAL_ROUTE_CACHE_SIZE = 2048
	MAX_NH_PER_ROUTE         = 32
	MAX_SCALE_ROUTES         = 100000
)
const (
	_ = iota
	RMgrOp_Add
	RMgrOp_Del
)

type IPRoute interface {
	// Get IP Type
	IPType() int
}
type IpPrefix struct {
	addr string //, mask string
}
type IPNextHop struct {
	weight int
	rifId  int32
}

type IpPrefixStateDBUpdateInfo struct {
	Op           int
	IpPrefixInfo IpPrefix
	//IpType       int
	//Ecmp         bool
}

const ( //pluginStatus
	inactive = iota
	configured
)

type PluginRouteStatus struct {
	status int
}
type IpRouteData struct {
	nwAddr           string
	nextHopMap       map[string]IPNextHop
	ecmpGrpId        uint64
	routeCreatedTime time.Time
	routeUpdatedTime time.Time
}
type PluginRouteData struct {
	pluginStatusMap map[PluginIntf]PluginRouteStatus
}
type PluginRouteReturnData struct {
	ipInfo        *pluginCommon.PluginIPRouteInfo
	inpInterface  interface{}
	retpluginIntf interface{}
	ret           int
}
type PluginRouteDataKey struct {
	IpPrefix  IpPrefix
	NextHopIp string
}
type RouteManager struct {
	nextHopMgr         *NextHopManager
	l3IntfMgr          *L3IntfManager
	ipRouteDB          map[IpPrefix]IpRouteData
	ipRouteDBKeys      []IpPrefix
	pluginRouteDB      map[PluginRouteDataKey]PluginRouteData
	logger             *logging.Writer
	plugins            []PluginIntf
	notifyChannel      *publisher.PubChannels
	routeOpChannel     chan *pluginCommon.PluginIPRouteInfo
	v4RouteCnt         int
	v6RouteCnt         int
	ecmpRouteCnt       int
	ipPrefixStateUpdCh chan IpPrefixStateDBUpdateInfo
}

//Route manager db
var RouteMgr RouteManager

func UpdateIPv4RouteDB(prefixIp, prefixMask, nextHopIp uint32) {
	/*
		routePrefix := Ipv4Prefix{
			addr: prefixIp,
			mask: prefixMask,
		}
		rEnt, exist := RouteMgr.ipv4RouteDB[routePrefix]
	*/
}

func (rMgr *RouteManager) Init(rsrcMgrs *ResourceManagers, bootMode int, pluginList []PluginIntf, logger *logging.Writer, notifyChannel *publisher.PubChannels) {
	rMgr.logger = logger
	rMgr.plugins = pluginList
	rMgr.nextHopMgr = rsrcMgrs.NextHopManager
	rMgr.l3IntfMgr = rsrcMgrs.L3IntfManager
	rMgr.notifyChannel = notifyChannel
	rMgr.ipRouteDB = make(map[IpPrefix]IpRouteData, INITIAL_ROUTE_CACHE_SIZE)
	rMgr.pluginRouteDB = make(map[PluginRouteDataKey]PluginRouteData, INITIAL_ROUTE_CACHE_SIZE)
	rMgr.routeOpChannel = make(chan *pluginCommon.PluginIPRouteInfo, 2*MAX_SCALE_ROUTES)
	rMgr.ipPrefixStateUpdCh = make(chan IpPrefixStateDBUpdateInfo, MAX_SCALE_ROUTES)
	if bootMode == pluginCommon.BOOT_MODE_WARMBOOT {
		//Restore route db during warm boot
		for _, plugin := range rMgr.plugins {
			if isPluginAsicDriver(plugin) == true {
				rv := plugin.RestoreIPv4RouteDB()
				if rv < 0 {
					rMgr.logger.Err("Failed to recover route db during warm boot")
				}
				/*				rv := plugin.RestoreIPv6RouteDB()
								if rv < 0 {
									rMgr.logger.Err("Failed to recover route db during warm boot")
								}*/
				break
			}
		}
	}
	go rMgr.RouteOpHandlerFunc()
	go rMgr.UpdateIpRouteStateDbInfo()
}

func (rMgr *RouteManager) RouteOpHandlerFunc() {
	for {
		select {
		case rtInfo := <-rMgr.routeOpChannel:
			if rtInfo.Op == RMgrOp_Add {
				rMgr.CreateIPRouteHdlr(rtInfo)
			} else if rtInfo.Op == RMgrOp_Del {
				rMgr.DeleteIPRouteHdlr(rtInfo)
			}
		}
	}
}

func (rMgr *RouteManager) CreateIPRoute(nwAddr string, ipAddr, networkMask, nextHopIp []uint8, dstIpType, nextHopIpType, weight, nextHopIfType int) error {
	rMgr.logger.Debug("AddCreateRouteOpToRMgr for nwAddr:", nwAddr, " ipAddr:", ipAddr, " networkMask:", networkMask, " nextHopIp:", nextHopIp, " dstIpType:", dstIpType, " nextHopIpType:", nextHopIpType, " weight:", weight, " nextHopIfType:", nextHopIfType)
	_, err := netutils.GetNetworkPrefixFromCIDR(nwAddr)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", nwAddr)
		return err
	}
	ipInfo, _, err := CreateIPObjects(nwAddr, ipAddr, networkMask, nextHopIp, dstIpType, nextHopIpType, weight, nextHopIfType)
	if err != nil {
		rMgr.logger.Err("CreateIPObjects returned error : err")
		return err
	}
	routerIfIndex, ifIndex, routeFlags, err := rMgr.GetRouterIfIndexInfo(ipInfo)
	if err != nil {
		rMgr.logger.Err("GetRouterIfIndexInfo return err:", err)
		return err
	}
	if ipInfo.IpType == syscall.AF_INET6 {
		routeFlags |= pluginCommon.ROUTE_TYPE_V6
	}
	ifName := ""
	if (routeFlags & pluginCommon.ROUTE_TYPE_NULL) != pluginCommon.ROUTE_TYPE_NULL {
		ifName, err = rMgr.l3IntfMgr.intfMgr.GetIfNameForIfIndex(ifIndex)
		if err != nil {
			rMgr.logger.Err("GetIfNameForIfIndex failed with error ", err, "for id ", routerIfIndex)
			return errors.New("IfName not found")
		}
	}
	ipInfo.RouteFlags = routeFlags
	ipInfo.RifId = routerIfIndex
	ipInfo.IfName = ifName
	ipInfo.Op = RMgrOp_Add
	rMgr.routeOpChannel <- ipInfo
	return nil
}
func (rMgr *RouteManager) DeleteIPRoute(nwAddr string, prefixIp, prefixMask, nextHopIp []uint8, dstIpType, nextHopIpType, nextHopIfType int) error {
	rMgr.logger.Debug("DelCreateRouteOpToRMgr called for prefixIp:", prefixIp, " prefixMask:", prefixMask, " nextHopIp:", nextHopIp, " nextHopIfType:", nextHopIfType)
	ipInfo, _, err := CreateIPObjects(nwAddr, prefixIp, prefixMask, nextHopIp, dstIpType, nextHopIpType, 0, nextHopIfType)
	if err != nil {
		rMgr.logger.Err(fmt.Sprintln("CreateIPObjects returned error : err"))
		return err
	}
	_, err = netutils.GetNetworkPrefixFromCIDR(nwAddr)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", nwAddr)
		return err
	}
	ipInfo.Op = RMgrOp_Del
	rMgr.routeOpChannel <- ipInfo
	return nil
}
func (rMgr *RouteManager) Deinit() {
	/* Currently no-op */
}

func (rMgr *RouteManager) GetNumV4Routes() int {
	return rMgr.v4RouteCnt
}

func (rMgr *RouteManager) GetNumV6Routes() int {
	return rMgr.v6RouteCnt
}

func (rMgr *RouteManager) GetNumECMPRoutes() int {
	return rMgr.ecmpRouteCnt
}

func CreateIPObjects(nwAddr string, prefixIp, maskedIp, nextHopIp []uint8, destIpType, nextHopIpType, weight, nextHopIfType int) (*pluginCommon.PluginIPRouteInfo, IPRoute, error) {
	var ipRoute IPRoute
	ipInfo := &pluginCommon.PluginIPRouteInfo{}
	ipInfo.NwAddr = nwAddr
	ipInfo.PrefixIp = prefixIp
	ipInfo.IpType = destIpType
	ipInfo.Mask = maskedIp
	ipInfo.NextHopIp = nextHopIp
	ipInfo.NextHopIpType = nextHopIpType
	ipInfo.NextHopIfType = nextHopIfType
	ipInfo.Weight = weight
	/*
		if ipInfo.IpType == syscall.AF_INET {
			ipRoute = &IPv4RouteObj{
				PrefixIp:      prefixIp,
				Mask:          maskedIp,
				NextHopIp:     nextHopIp,
				NextHopIfType: nextHopIfType,
				NextHopIpType: nextHopIpType,
				Weight:        weight,
			}
		} else {
			ipRoute = &IPv6RouteObj{
				PrefixIp:      prefixIp,
				Mask:          maskedIp,
				NextHopIp:     nextHopIp,
				NextHopIfType: nextHopIfType,
				NextHopIpType: nextHopIpType,
				Weight:        weight,
			}
		}
	*/
	return ipInfo, ipRoute, nil
}
func (rMgr *RouteManager) GetRouterIfIndexInfo(ipRoute *pluginCommon.PluginIPRouteInfo) (routerIfIndex int32, ifIndex int32, routeFlags uint32, err error) {
	/* Check if nexthop IP resides in a directly attached subnet */
	rMgr.logger.Info("GetRouterIfIndexInfo ,nextHopIfType:", ipRoute.NextHopIfType)
	if ipRoute.NextHopIfType == commonDefs.IfTypeNull {
		rMgr.logger.Info("Setting null route flag")
		routeFlags |= pluginCommon.ROUTE_TYPE_NULL
		return routerIfIndex, ifIndex, routeFlags, nil
	}
	if pluginCommon.IsZeroIP(ipRoute.NextHopIp, ipRoute.NextHopIpType) {
		if ipRoute.IpType == syscall.AF_INET {
			routerIfIndex, ifIndex = rMgr.l3IntfMgr.GetRouterIfIndexContainingNeighbor(ipRoute.PrefixIp)
		} else {
			routerIfIndex, ifIndex = rMgr.l3IntfMgr.GetRouterIfIndexContainingv6Neighbor(ipRoute.PrefixIp)
		}
		//rMgr.logger.Info("setting to connected")
		routeFlags |= pluginCommon.ROUTE_TYPE_CONNECTED
	} else {
		if ipRoute.NextHopIpType == syscall.AF_INET {
			routerIfIndex, ifIndex = rMgr.l3IntfMgr.GetRouterIfIndexContainingNeighbor(ipRoute.NextHopIp)
		} else {
			routerIfIndex, ifIndex = rMgr.l3IntfMgr.GetRouterIfIndexContainingv6Neighbor(ipRoute.NextHopIp)
		}
		if routerIfIndex == pluginCommon.INVALID_IFINDEX {
			return routerIfIndex, ifIndex, routeFlags, errors.New("Cannot locate router interface containing route nexthop IP")
		}
	}
	return routerIfIndex, ifIndex, routeFlags, nil
}

func (rMgr *RouteManager) UpdateIpRouteStateDbInfo() {
	for {
		select {
		case ipPrefixChInfo, ok := <-rMgr.ipPrefixStateUpdCh:
			if !ok {
				continue
			}
			switch ipPrefixChInfo.Op {
			case RMgrOp_Add:
				rMgr.ipRouteDBKeys = append(rMgr.ipRouteDBKeys, ipPrefixChInfo.IpPrefixInfo)
			case RMgrOp_Del:
				for idx, val := range rMgr.ipRouteDBKeys {
					if val == ipPrefixChInfo.IpPrefixInfo {
						rMgr.ipRouteDBKeys = append(rMgr.ipRouteDBKeys[:idx], rMgr.ipRouteDBKeys[idx+1:]...)
						break
					}
				}
			}
		}
	}
}

func (rMgr *RouteManager) CreateIPRouteHdlr(ipInfo *pluginCommon.PluginIPRouteInfo) {
	var existingRouteEntry, ecmpRoute bool
	var rEnt IpRouteData
	var nhObjId uint64
	var oldNhIp string

	prefix, err := netutils.GetNetworkPrefixFromCIDR(ipInfo.NwAddr)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", ipInfo.NwAddr)
		return
	}
	ipPrefix := IpPrefix{addr: string(prefix)}

	nextHopIpStr := (net.IP(ipInfo.NextHopIp)).String()
	rMgr.logger.Debug("In create nextHopIpStr:", nextHopIpStr, "ipPrefix:", nextHopIpStr)

	rEnt, existingRouteEntry = rMgr.ipRouteDB[ipPrefix]
	rMgr.logger.Debug("CreateIPRouteHdlr is acquiring Prefix:", ipPrefix, "for nexthopIpstr:", nextHopIpStr)

	if existingRouteEntry {
		ecmpRoute = true
		ipInfo.RouteFlags |= pluginCommon.ROUTE_OPERATION_TYPE_UPDATE
		rMgr.logger.Debug("Route exists in internal db for ipPrefix:", ipPrefix)
		if _, existingNhEntry := rEnt.nextHopMap[nextHopIpStr]; existingNhEntry {
			return
		}
		rEnt.nextHopMap[nextHopIpStr] = IPNextHop{weight: ipInfo.Weight, rifId: ipInfo.RifId}
		//Touch route updated timestamp
		rEnt.routeUpdatedTime = time.Now()
		ipInfo.RouteFlags |= pluginCommon.ROUTE_TYPE_MULTIPATH
		var nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo
		for ip, val := range rEnt.nextHopMap {
			if string(ip) != nextHopIpStr {
				oldNhIp = string(ip)
				rMgr.logger.Debug("string(ip):", string(ip), " not equal to nextHopIpStr:", nextHopIpStr, " so oldNhIp:", oldNhIp)
			}
			nhGroupMembers = append(nhGroupMembers, &pluginCommon.NextHopGroupMemberInfo{
				IpAddr: ip,
				Weight: val.weight,
				RifId:  val.rifId,
			})
		}
		rMgr.logger.Debug("oldNhIp:", oldNhIp, "ecmpGrpId:", rEnt.ecmpGrpId)
		if rEnt.ecmpGrpId == pluginCommon.INVALID_OBJECT_ID {
			nhObjId, err = rMgr.nextHopMgr.CreateNextHopGroup(nhGroupMembers)
			if err != nil {
				rMgr.logger.Err("Failed to create next hop group when adding route")
				return
			}
			rEnt.ecmpGrpId = nhObjId
			nbrInfo := &pluginCommon.PluginIPNeighborInfo{
				IfIndex:       pluginCommon.INVALID_IFINDEX,
				MacAddr:       net.HardwareAddr{0, 0, 0, 0, 0, 0},
				VlanId:        pluginCommon.MAX_VLAN_ID,
				OperationType: RT_NBR_DELETE_OPERATION,
				Address:       oldNhIp,
				IpType:        ipInfo.NextHopIpType,
			}
			rMgr.logger.Debug("calling DeleteNextHop for:", *nbrInfo)
			//Delete ref to singleton nexthop
			err = rMgr.nextHopMgr.DeleteNextHop(nbrInfo)
			if err != nil {
				rMgr.logger.Err("DeleteNextHop failed during create ip route for:", *nbrInfo, "err:", err)
			}
		} else {
			nhObjId, err = rMgr.nextHopMgr.UpdateNextHopGroup(rEnt.ecmpGrpId, nhGroupMembers)
			if err != nil {
				rMgr.logger.Err("Failed to update next hop group when adding route")
				return
			}
			rEnt.ecmpGrpId = nhObjId
		}
	} else {
		rEnt.nwAddr = ipInfo.NwAddr
		rEnt.routeCreatedTime = time.Now()
		rEnt.ecmpGrpId = pluginCommon.INVALID_OBJECT_ID
		rEnt.nextHopMap = make(map[string]IPNextHop, MAX_NH_PER_ROUTE)
		rEnt.nextHopMap[nextHopIpStr] = IPNextHop{weight: ipInfo.Weight, rifId: ipInfo.RifId}
		ipInfo.RouteFlags |= pluginCommon.ROUTE_TYPE_SINGLEPATH
		//Create a next hop only if this is not a connected route and it is not a null route
		if ((ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_CONNECTED) != pluginCommon.ROUTE_TYPE_CONNECTED) &&
			((ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_NULL) != pluginCommon.ROUTE_TYPE_NULL) {
			rMgr.logger.Info("not a connected route and not a null route, create next hop")
			vlanId := pluginCommon.GetIdFromIfIndex(ipInfo.RifId)
			nbrInfo := &pluginCommon.PluginIPNeighborInfo{
				NextHopFlags:  pluginCommon.NEXTHOP_TYPE_COPY_TO_CPU,
				IfIndex:       pluginCommon.INVALID_IFINDEX,
				MacAddr:       net.HardwareAddr{0, 0, 0, 0, 0, 0},
				VlanId:        vlanId,
				OperationType: RT_NBR_CREATE_OPERATION,
				IpType:        ipInfo.NextHopIpType,
				Address:       nextHopIpStr,
			}
			rMgr.logger.Debug("calling CreateNextHop for:", *nbrInfo)
			err = rMgr.nextHopMgr.CreateNextHop(&nbrInfo)
			if err != nil {
				rMgr.logger.Err("Failed to create next hop when adding route")
				return
			}
			nhObjId = nbrInfo.NextHopId
			rMgr.logger.Debug("returned next hop id is:", nbrInfo.NextHopId)
		}
	}
	for idx, _ := range ipInfo.PrefixIp {
		ipInfo.PrefixIp[idx] = ipInfo.PrefixIp[idx] & ipInfo.Mask[idx]
	}
	ipInfo.NextHopId = nhObjId
	ipInfo.Op = pluginCommon.PluginOp_Add
	for _, plugin := range rMgr.plugins {
		//uncomment this for scale testing
		rMgr.logger.Debug("adding route create to plugin ", plugin)
		plugin.AddToRouteChannel(ipInfo)
		//rMgr.logger.Debug("return from route create from plugin ", plugin)
		/*
			rv := plugin.CreateIPRoute(ipInfo)
			if rv < 0 {
				//TO-DO: Uncomment this return code once asicd profiling is supported to be able to support 30k routes
				//rMgr.logger.Err("Failed to install route")
				// @TODO: madhavi: Un-comment delete also as we do not want to hold on stale rEnt information for an
				// ipPrefix
				//delete(rMgr.ipRouteDB, ipPrefix)
				//return errors.New("Failed to add IPv4 route")
			}
		*/
		//rMgr.logger.Debug("Installed the route successfully :")	}
	}
	//Insert into key cache new keys only
	if !existingRouteEntry {
		rMgr.ipPrefixStateUpdCh <- IpPrefixStateDBUpdateInfo{RMgrOp_Add, ipPrefix}
	}
	//Update route db sw cache
	rMgr.ipRouteDB[ipPrefix] = rEnt
	if ipInfo.IpType == syscall.AF_INET {
		rMgr.v4RouteCnt += 1
	} else {
		rMgr.v6RouteCnt += 1
	}
	if ecmpRoute {
		rMgr.ecmpRouteCnt += 1
	}
	return
}

func (rMgr *RouteManager) DeleteIPRouteHdlr(ipInfo *pluginCommon.PluginIPRouteInfo) {

	var rEnt IpRouteData
	var exist bool
	var routeFlags uint32
	prefix, err := netutils.GetNetworkPrefixFromCIDR(ipInfo.NwAddr)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", ipInfo.NwAddr)
		return
	}
	rMgr.logger.Debug("In Delete prefix:", prefix, " string(prefix):", string(prefix))
	ipPrefix := IpPrefix{addr: string(prefix)}
	rEnt, exist = rMgr.ipRouteDB[ipPrefix]
	if !exist {
		rMgr.logger.Err("Entry not found during delete operation for ipPrefix:", ipPrefix)
		return
	}
	nextHopIpStr := (net.IP(ipInfo.NextHopIp)).String()

	rMgr.logger.Debug("looking in nexthopmap for nextHopIp:", nextHopIpStr)
	if _, exist = rEnt.nextHopMap[nextHopIpStr]; !exist {
		rMgr.logger.Err("NextHop Entry not found for the given route during delete operation")
		return
	}
	if pluginCommon.IsZeroIP(ipInfo.NextHopIp, ipInfo.NextHopIpType) {
		routeFlags |= pluginCommon.ROUTE_TYPE_CONNECTED
	}
	if ipInfo.IpType == syscall.AF_INET6 {
		routeFlags |= pluginCommon.ROUTE_TYPE_V6
	}
	if ipInfo.NextHopIfType == commonDefs.IfTypeNull {
		rMgr.logger.Info("Setting null route flag")
		routeFlags |= pluginCommon.ROUTE_TYPE_NULL
	}
	delete(rEnt.nextHopMap, nextHopIpStr)
	numOfNextHops := len(rEnt.nextHopMap)
	rMgr.logger.Debug("deleted nextHopMap for ", nextHopIpStr, " numOfNextHops:", numOfNextHops)
	for idx, _ := range ipInfo.PrefixIp {
		ipInfo.PrefixIp[idx] = ipInfo.PrefixIp[idx] & ipInfo.Mask[idx]
	}
	ipInfo.RouteFlags = routeFlags
	ipInfo.Op = pluginCommon.PluginOp_Del
	ipInfo.DoneChannel = make(chan int, len(rMgr.plugins))
	nbrInfo := &pluginCommon.PluginIPNeighborInfo{
		IfIndex:       pluginCommon.INVALID_IFINDEX,
		MacAddr:       net.HardwareAddr{0, 0, 0, 0, 0, 0},
		VlanId:        pluginCommon.MAX_VLAN_ID,
		OperationType: RT_NBR_DELETE_OPERATION,
		IpType:        ipInfo.NextHopIpType,
		Address:       nextHopIpStr,
	}
	//Determine nh ref count/nhGroup ref count to determine if we have to block on route del call
	var nhRefCount int
	if rEnt.ecmpGrpId == pluginCommon.INVALID_OBJECT_ID {
		nhRefCount = rMgr.nextHopMgr.GetNextHopRefCount(nbrInfo)
	} else {
		nhRefCount = rMgr.nextHopMgr.GetNextHopGroupRefCount(rEnt.ecmpGrpId)
	}
	rMgr.logger.Debug("DeleteIPRoute Prefix - ", ipInfo.NwAddr, " NextHop - ", nextHopIpStr, " RefCnt = ", nhRefCount)
	for _, plugin := range rMgr.plugins {
		/* Call delete route for asic plugin only if num nexthops = 0*/
		if (numOfNextHops == 0 && isPluginAsicDriver(plugin)) || !isPluginAsicDriver(plugin) {
			if nhRefCount > 1 {
				ipInfo.DoneChannel = nil
			}
			plugin.AddToRouteChannel(ipInfo)
			if isPluginAsicDriver(plugin) && nhRefCount <= 1 {
				<-ipInfo.DoneChannel
			}
		}
	}
	if numOfNextHops == 0 {
		//delete a next hop only if this is not a connected route and it is not a null route
		if ((ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_CONNECTED) != pluginCommon.ROUTE_TYPE_CONNECTED) &&
			((ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_NULL) != pluginCommon.ROUTE_TYPE_NULL) {
			if rEnt.ecmpGrpId == pluginCommon.INVALID_OBJECT_ID {
				rMgr.logger.Debug("Calling delete Next hop for:", *nbrInfo)
				err := rMgr.nextHopMgr.DeleteNextHop(nbrInfo)
				if err != nil {
					rMgr.logger.Err("Failed to delete next hop when deleting route", *nbrInfo)
				}
			} else {
				var nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo
				for ip, val := range rEnt.nextHopMap {
					nhGroupMembers = append(nhGroupMembers, &pluginCommon.NextHopGroupMemberInfo{
						IpAddr: ip,
						Weight: val.weight,
					})
				}
				rMgr.logger.Debug("Calling DeleteNextHopGroup for ecmpGrpId:", rEnt.ecmpGrpId)
				err := rMgr.nextHopMgr.DeleteNextHopGroup(rEnt.ecmpGrpId)
				if err != nil {
					rMgr.logger.Err("Failed to delete next hop group when deleting route for ecmpGrpId", rEnt.ecmpGrpId)
				} else {
					//Decrement ecmp route count
					rMgr.ecmpRouteCnt -= 1
				}
			}
		}
		//Delete flom route db sw cache
		rMgr.logger.Debug("deleting ipPrefix:", ipPrefix, "from ipRouteDB")
		delete(rMgr.ipRouteDB, ipPrefix)
		rMgr.ipPrefixStateUpdCh <- IpPrefixStateDBUpdateInfo{RMgrOp_Del, ipPrefix}
	} else {
		var nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo
		var err error
		for ip, val := range rEnt.nextHopMap {
			nhGroupMembers = append(nhGroupMembers, &pluginCommon.NextHopGroupMemberInfo{
				IpAddr: ip,
				Weight: val.weight,
			})
		}
		rMgr.logger.Debug("Calling UpdateNextHopGroup for ecmpGrpId:", rEnt.ecmpGrpId)
		grpId, err := rMgr.nextHopMgr.UpdateNextHopGroup(rEnt.ecmpGrpId, nhGroupMembers)
		if err != nil {
			rMgr.logger.Err("Failed to update next hop group when adding route")
		}
		/*
		  If update to NhGroup yields new group id then :
		  - Update route to point to new ecmp grp
		  - Delete old ecmp grp
		*/
		if grpId != rEnt.ecmpGrpId {
			rtInfo := new(pluginCommon.PluginIPRouteInfo)
			*rtInfo = *ipInfo
			rtInfo.RouteFlags = pluginCommon.ROUTE_OPERATION_TYPE_UPDATE | pluginCommon.ROUTE_TYPE_MULTIPATH
			if ipInfo.IpType == syscall.AF_INET6 {
				rtInfo.RouteFlags |= pluginCommon.ROUTE_TYPE_V6
			}
			rtInfo.NextHopId = grpId
			rtInfo.Op = pluginCommon.PluginOp_Add
			rtInfo.DoneChannel = make(chan int, 1)
			for _, plugin := range rMgr.plugins {
				if isPluginAsicDriver(plugin) {
					plugin.AddToRouteChannel(rtInfo)
					<-rtInfo.DoneChannel
				}
			}
			//Delete old NhGroup if this was the last route pointing to it
			if nhRefCount == 1 {
				err = rMgr.nextHopMgr.DeleteNextHopGroup(rEnt.ecmpGrpId)
				if err != nil {
					rMgr.logger.Err("Failed to delete old next hop group after updating next hop group in delete route", rEnt.ecmpGrpId)
				}
			}
		}
		rEnt.ecmpGrpId = grpId
		rMgr.ipRouteDB[ipPrefix] = rEnt
	}
	if ipInfo.IpType == syscall.AF_INET {
		rMgr.v4RouteCnt -= 1
	} else {
		rMgr.v6RouteCnt -= 1
	}
	return
}
func (rMgr *RouteManager) GetIPRoute(destinationNw string) (*asicdInt.IPRouteHwState, error) {
	prefix, err := netutils.GetNetworkPrefixFromCIDR(destinationNw)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", destinationNw)
		return nil, err
	}
	rMgr.logger.Info("prefix:", prefix, " string(prefix):", string(prefix))
	ipPrefix := IpPrefix{addr: string(prefix)}
	if rEnt, exist := rMgr.ipRouteDB[ipPrefix]; exist {
		routeObj := new(asicdInt.IPRouteHwState)
		routeObj.DestinationNw = rEnt.nwAddr
		routeObj.RouteCreatedTime = rEnt.routeCreatedTime.String()
		routeObj.RouteUpdatedTime = rEnt.routeUpdatedTime.String()
		for nextHopIp, _ := range rEnt.nextHopMap {
			routeObj.NextHopIps += nextHopIp + "," //fmt.Sprintf("%d.%d.%d.%d,", byte(nextHopIp>>24), byte(nextHopIp>>16), byte(nextHopIp>>8), byte(nextHopIp))
		}
		strings.TrimRight(routeObj.NextHopIps, ",")
		return routeObj, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unable to locate Hw Route entry corresponding to key - ", destinationNw))
	}
	return nil, nil
}

//FIXME: Add logic to garbage collect keys slice maintained for getbulk
func (rMgr *RouteManager) GetBulkIPRoute(start, count int) (end, listLen int, more bool, routeList []*asicdInt.IPRouteHwState) {
	var numEntries, idx int = 0, 0
	if len(rMgr.ipRouteDB) == 0 {
		return 0, 0, false, nil
	}
	for idx = start; idx < len(rMgr.ipRouteDBKeys); idx++ {
		rMgr.logger.Debug("rMgr.ipRouteDBKeys[idx] = ", rMgr.ipRouteDBKeys[idx])
		if rEnt, ok := rMgr.ipRouteDB[rMgr.ipRouteDBKeys[idx]]; ok {
			var routeObj asicdInt.IPRouteHwState
			if numEntries == count {
				more = true
				break
			}
			routeObj.DestinationNw = rEnt.nwAddr //fmt.Sprintf("%d.%d.%d.%d/%d", byte(rMgr.ipv4RouteDBKeys[idx].addr>>24), byte(rMgr.ipv4RouteDBKeys[idx].addr>>16), byte(rMgr.ipv4RouteDBKeys[idx].addr>>8), byte(rMgr.ipv4RouteDBKeys[idx].addr), maskLen)
			routeObj.RouteCreatedTime = rEnt.routeCreatedTime.String()
			routeObj.RouteUpdatedTime = rEnt.routeUpdatedTime.String()
			for nextHopIp, _ := range rEnt.nextHopMap {
				routeObj.NextHopIps += nextHopIp + "," //fmt.Sprintf("%d.%d.%d.%d,", byte(nextHopIp>>24), byte(nextHopIp>>16), byte(nextHopIp>>8), byte(nextHopIp))
			}
			strings.TrimRight(routeObj.NextHopIps, ",")
			routeList = append(routeList, &routeObj)
			numEntries += 1
		}
	}
	end = idx
	if end == len(rMgr.ipRouteDBKeys) {
		more = false
	}
	return end, len(routeList), more, routeList
}

func (rMgr *RouteManager) GetIPv4Route(destinationNw string) (*asicdServices.IPv4RouteHwState, error) {
	if !netutils.IsIPv4Addr(destinationNw) {
		rMgr.logger.Debug("destinationNw ", destinationNw, " not IPv4 Address")
		return nil, errors.New(fmt.Sprintln("Incorrect state object queried for getting route Info for:", destinationNw, " try IPv6RouteHw"))
	}
	prefix, err := netutils.GetNetworkPrefixFromCIDR(destinationNw)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", destinationNw)
		return nil, err
	}
	rMgr.logger.Info("prefix:", prefix, " string(prefix):", string(prefix))
	ipPrefix := IpPrefix{addr: string(prefix)}
	if rEnt, exist := rMgr.ipRouteDB[ipPrefix]; exist {
		routeObj := new(asicdServices.IPv4RouteHwState)
		routeObj.DestinationNw = rEnt.nwAddr
		routeObj.RouteCreatedTime = rEnt.routeCreatedTime.String()
		routeObj.RouteUpdatedTime = rEnt.routeUpdatedTime.String()
		for nextHopIp, _ := range rEnt.nextHopMap {
			routeObj.NextHopIps += nextHopIp + "," //fmt.Sprintf("%d.%d.%d.%d,", byte(nextHopIp>>24), byte(nextHopIp>>16), byte(nextHopIp>>8), byte(nextHopIp))
		}
		strings.TrimRight(routeObj.NextHopIps, ",")
		return routeObj, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unable to locate Hw Route entry corresponding to key - ", destinationNw))
	}
	return nil, nil
}

//FIXME: Add logic to garbage collect keys slice maintained for getbulk
func (rMgr *RouteManager) GetBulkIPv4Route(start, count int) (end, listLen int, more bool, routeList []*asicdServices.IPv4RouteHwState) {
	var numEntries, idx int = 0, 0
	if len(rMgr.ipRouteDB) == 0 {
		return 0, 0, false, nil
	}
	for idx = start; idx < len(rMgr.ipRouteDBKeys); idx++ {
		rMgr.logger.Debug("rMgr.ipRouteDBKeys[idx] = ", rMgr.ipRouteDBKeys[idx])
		if rEnt, ok := rMgr.ipRouteDB[rMgr.ipRouteDBKeys[idx]]; ok {
			var routeObj asicdServices.IPv4RouteHwState
			if numEntries == count {
				more = true
				break
			}
			if !netutils.IsIPv4Addr(rEnt.nwAddr) {
				rMgr.logger.Debug("destinationNw ", rEnt.nwAddr, " not IPv4 Address, query the next entry")
				continue
			}
			routeObj.DestinationNw = rEnt.nwAddr //fmt.Sprintf("%d.%d.%d.%d/%d", byte(rMgr.ipv4RouteDBKeys[idx].addr>>24), byte(rMgr.ipv4RouteDBKeys[idx].addr>>16), byte(rMgr.ipv4RouteDBKeys[idx].addr>>8), byte(rMgr.ipv4RouteDBKeys[idx].addr), maskLen)
			routeObj.RouteCreatedTime = rEnt.routeCreatedTime.String()
			routeObj.RouteUpdatedTime = rEnt.routeUpdatedTime.String()
			for nextHopIp, _ := range rEnt.nextHopMap {
				routeObj.NextHopIps += nextHopIp + "," //fmt.Sprintf("%d.%d.%d.%d,", byte(nextHopIp>>24), byte(nextHopIp>>16), byte(nextHopIp>>8), byte(nextHopIp))
			}
			strings.TrimRight(routeObj.NextHopIps, ",")
			routeList = append(routeList, &routeObj)
			numEntries += 1
		}
	}
	end = idx
	if end == len(rMgr.ipRouteDBKeys) {
		more = false
	}
	return end, len(routeList), more, routeList
}

func (rMgr *RouteManager) GetIPv6Route(destinationNw string) (*asicdServices.IPv6RouteHwState, error) {
	if !netutils.IsIPv6Addr(destinationNw) {
		rMgr.logger.Debug("destinationNw ", destinationNw, " not IPv6 Address")
		return nil, errors.New(fmt.Sprintln("Incorrect state object queried for getting route Info for:", destinationNw, " try IPv4RouteHw"))
	}
	prefix, err := netutils.GetNetworkPrefixFromCIDR(destinationNw)
	if err != nil {
		rMgr.logger.Err("Invalid destination prefix:", destinationNw)
		return nil, err
	}
	rMgr.logger.Info("prefix:", prefix, " string(prefix):", string(prefix))
	ipPrefix := IpPrefix{addr: string(prefix)}
	if rEnt, exist := rMgr.ipRouteDB[ipPrefix]; exist {
		routeObj := new(asicdServices.IPv6RouteHwState)
		routeObj.DestinationNw = rEnt.nwAddr
		routeObj.RouteCreatedTime = rEnt.routeCreatedTime.String()
		routeObj.RouteUpdatedTime = rEnt.routeUpdatedTime.String()
		for nextHopIp, _ := range rEnt.nextHopMap {
			routeObj.NextHopIps += nextHopIp + "," //fmt.Sprintf("%d.%d.%d.%d,", byte(nextHopIp>>24), byte(nextHopIp>>16), byte(nextHopIp>>8), byte(nextHopIp))
		}
		strings.TrimRight(routeObj.NextHopIps, ",")
		return routeObj, nil
	} else {
		return nil, errors.New(fmt.Sprintf("Unable to locate Hw Route entry corresponding to key - ", destinationNw))
	}
	return nil, nil
}

//FIXME: Add logic to garbage collect keys slice maintained for getbulk
func (rMgr *RouteManager) GetBulkIPv6Route(start, count int) (end, listLen int, more bool, routeList []*asicdServices.IPv6RouteHwState) {
	var numEntries, idx int = 0, 0
	if len(rMgr.ipRouteDB) == 0 {
		return 0, 0, false, nil
	}
	for idx = start; idx < len(rMgr.ipRouteDBKeys); idx++ {
		rMgr.logger.Debug("rMgr.ipRouteDBKeys[idx] = ", rMgr.ipRouteDBKeys[idx])
		if rEnt, ok := rMgr.ipRouteDB[rMgr.ipRouteDBKeys[idx]]; ok {
			var routeObj asicdServices.IPv6RouteHwState
			if numEntries == count {
				more = true
				break
			}
			if !netutils.IsIPv6Addr(rEnt.nwAddr) {
				rMgr.logger.Debug("destinationNw ", rEnt.nwAddr, " not IPv6 Address, query the next entry")
				continue
			}
			routeObj.DestinationNw = rEnt.nwAddr //fmt.Sprintf("%d.%d.%d.%d/%d", byte(rMgr.ipv4RouteDBKeys[idx].addr>>24), byte(rMgr.ipv4RouteDBKeys[idx].addr>>16), byte(rMgr.ipv4RouteDBKeys[idx].addr>>8), byte(rMgr.ipv4RouteDBKeys[idx].addr), maskLen)
			routeObj.RouteCreatedTime = rEnt.routeCreatedTime.String()
			routeObj.RouteUpdatedTime = rEnt.routeUpdatedTime.String()
			for nextHopIp, _ := range rEnt.nextHopMap {
				routeObj.NextHopIps += nextHopIp + "," //fmt.Sprintf("%d.%d.%d.%d,", byte(nextHopIp>>24), byte(nextHopIp>>16), byte(nextHopIp>>8), byte(nextHopIp))
			}
			strings.TrimRight(routeObj.NextHopIps, ",")
			routeList = append(routeList, &routeObj)
			numEntries += 1
		}
	}
	end = idx
	if end == len(rMgr.ipRouteDBKeys) {
		more = false
	}
	return end, len(routeList), more, routeList
}

/*
func DeleteRouteRetHandler(ipInfo *pluginCommon.PluginIPRouteInfo, inpInterface interface{}, retpluginIntf interface{}, ret int) {
	if inpInterface == nil {
		fmt.Println("inpInterface nil")
		return
	}
	rMgr := inpInterface.(*RouteManager)
	ipPrefix := ipInfo.RouteDBKeys.(IpPrefix)
	if ret != 0 {
		rMgr.logger.Err("delete route failed ")
		return
	}
	var exist bool
	if _, exist = rMgr.ipRouteDB[ipPrefix]; !exist {
		rMgr.logger.Err("Entry not found during delete operation for ipPrefix:", ipPrefix)
		return
	}
	nextHopIp := (net.IP(ipInfo.NextHopIp)).String()
	pluginRouteDataKey := PluginRouteDataKey{ipPrefix, nextHopIp}
	if retpluginIntf != nil {
		var pluginRouteInfo PluginRouteData
		retplugin := retpluginIntf.(PluginIntf)
		rMgr.logger.Debug("Delete route ret value for ipInfo: ", *ipInfo, ":", ret, " from plugin:", retplugin)
		pluginRouteInfo, exist = rMgr.pluginRouteDB[pluginRouteDataKey]
		rMgr.logger.Debug("DeleteRouteRetHandler():pluginrouteDb for key:", pluginRouteDataKey, " exist:", exist)
		if exist {
			pluginRouteStatusMap := pluginRouteInfo.pluginStatusMap
			_, chk := pluginRouteStatusMap[retplugin]
			rMgr.logger.Debug("DeleteRouteRetHandler(),chk:", chk, " len(pluginRouteStatusMap):", len(pluginRouteStatusMap))
			if chk {
				delete(pluginRouteStatusMap, retplugin)
				pluginRouteInfo.pluginStatusMap = pluginRouteStatusMap
				rMgr.pluginRouteDB[pluginRouteDataKey] = pluginRouteInfo

				for _, plugin := range rMgr.plugins {
					if plugin == retplugin {
						continue
					}
					if _, ok := pluginRouteInfo.pluginStatusMap[plugin]; !ok {
						continue
					}
					if pluginRouteInfo.pluginStatusMap[plugin].status == configured {
						//there is a plugin on which this route prefix still is not deleted
						return
					}
				}
			}
		}
	}

	rEnt := ipInfo.RouteEntity.(IpRouteData)
	rMgr.logger.Debug("DeleteRouteRetHandler():,ecmpgrpid:", rEnt.ecmpGrpId)
	numOfNextHops := len(rEnt.nextHopMap)
	if numOfNextHops == 0 {
		//Remove key from cache
		for idx, val := range rMgr.ipRouteDBKeys {
			if val == ipPrefix {
				rMgr.ipRouteDBKeys = append(rMgr.ipRouteDBKeys[:idx], rMgr.ipRouteDBKeys[idx+1:]...)
				break
			}
		}
		//Delete from route db sw cache
		delete(rMgr.ipRouteDB, ipPrefix)
		//delete a next hop only if this is not a connected route and it is not a null route
		if ((ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_CONNECTED) != pluginCommon.ROUTE_TYPE_CONNECTED) && ((ipInfo.RouteFlags & pluginCommon.ROUTE_TYPE_NULL) != pluginCommon.ROUTE_TYPE_NULL) {
			if rEnt.ecmpGrpId == pluginCommon.INVALID_OBJECT_ID {
				nbrInfo := &pluginCommon.PluginIPNeighborInfo{
					IfIndex:       pluginCommon.INVALID_IFINDEX,
					MacAddr:       net.HardwareAddr{0, 0, 0, 0, 0, 0},
					VlanId:        pluginCommon.MAX_VLAN_ID,
					OperationType: RT_NBR_DELETE_OPERATION,
					IpType:        ipInfo.NextHopIpType,
					Address:       (net.IP(ipInfo.NextHopIp)).String(),
				}
				err := rMgr.nextHopMgr.DeleteNextHop(nbrInfo)
				if err != nil {
					rMgr.logger.Err("Failed to delete next hop when deleting route")
					return
				}
			} else {
				var nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo
				for ip, val := range rMgr.ipRouteDB[ipPrefix].nextHopMap {
					nhGroupMembers = append(nhGroupMembers, &pluginCommon.NextHopGroupMemberInfo{
						IpAddr: ip,
						Weight: val.weight,
					})
				}
				err := rMgr.nextHopMgr.DeleteNextHopGroup(rEnt.ecmpGrpId)
				if err != nil {
					rMgr.logger.Err("Failed to delete next hop group when deleting route")
					return
				}
				//Decrement ecmp route count
				rMgr.ecmpRouteCnt -= 1
			}
		}
	} else {
		var nhGroupMembers []*pluginCommon.NextHopGroupMemberInfo
		var err error
		for ip, val := range rMgr.ipRouteDB[ipPrefix].nextHopMap {
			nhGroupMembers = append(nhGroupMembers, &pluginCommon.NextHopGroupMemberInfo{
				IpAddr: ip,
				Weight: val.weight,
			})
		}
		rEnt.ecmpGrpId, err = rMgr.nextHopMgr.UpdateNextHopGroup(rEnt.ecmpGrpId, nhGroupMembers)
		if err != nil {
			rMgr.logger.Err("Failed to update next hop group when adding route")
			return
		}
		rMgr.ipRouteDB[ipPrefix] = rEnt
	}
	if ipInfo.IpType == syscall.AF_INET {
		rMgr.v4RouteCnt -= 1
	} else {
		rMgr.v6RouteCnt -= 1
	}
}
func CreateRouteRetHandler(ipInfo *pluginCommon.PluginIPRouteInfo, inpInterface interface{}, retpluginIntf interface{}, ret int) {
	if inpInterface == nil || ipInfo == nil || retpluginIntf == nil {
		fmt.Println("inpInterface/ipInfo/retpluginIntf in CreateRouteRetHandler() nil")
		return
	}
	rMgr := inpInterface.(*RouteManager)
	ipPrefix := ipInfo.RouteDBKeys.(IpPrefix)
	retplugin := retpluginIntf.(PluginIntf)
	nextHopIp := (net.IP(ipInfo.NextHopIp)).String()
	pluginRouteDataKey := PluginRouteDataKey{ipPrefix, nextHopIp}
	rMgr.logger.Debug("Create route ret value for ipInfo: ", *ipInfo, ":", ret, " from plugin:", retplugin)
	if ret != 0 {
		rMgr.logger.Err("Create route ret value for ipInfo: ", *ipInfo, ":", ret, " from plugin:", retplugin)
		var rEnt IpRouteData
		var exist bool
		var pluginRouteInfo PluginRouteData
		msgBuf, err := json.Marshal(ipInfo.PrefixIp)
		if err != nil {
			rMgr.logger.Err("Failed to marshal route create failure message")
			return
		}
		notification := asicdCommonDefs.AsicdNotification{
			MsgType: uint8(asicdCommonDefs.NOTIFY_IPV4_ROUTE_CREATE_FAILURE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			rMgr.logger.Err("Failed to marshal route create failure message")
			return
		}
		rMgr.notifyChannel.Ribd <- notificationBuf
		//check if there is a pluginRouteInfo for this prefix
		pluginRouteInfo, exist = rMgr.pluginRouteDB[pluginRouteDataKey]
		if !exist {
			rMgr.logger.Debug("pluginRouteDbEntry not found during create fail cleanup for ipPrefix:", pluginRouteDataKey)
		}
		pluginRouteStatusMap := pluginRouteInfo.pluginStatusMap
		if pluginRouteStatusMap == nil {
			pluginRouteStatusMap = make(map[PluginIntf]PluginRouteStatus)
		}
		//update the routecreated plugin status for this prefix for this plugin
		var pluginRouteStatus PluginRouteStatus
		pluginRouteStatus.status = inactive
		pluginRouteStatusMap[retplugin] = pluginRouteStatus
		pluginRouteInfo.pluginStatusMap = pluginRouteStatusMap
		rMgr.pluginRouteDB[pluginRouteDataKey] = pluginRouteInfo

		if rEnt, exist = rMgr.ipRouteDB[ipPrefix]; !exist {
			rMgr.logger.Info("Entry not found during create fail cleanup for ipPrefix:", ipPrefix)
			return
		}
		rMgr.logger.Debug("looking in nexthopmap for nextHopIp:", nextHopIp)
		if _, exist = rEnt.nextHopMap[nextHopIp]; !exist {
			rMgr.logger.Err("NextHop Entry not found for the given route during create failure cleanup operation")
			return
		}
		delete(rEnt.nextHopMap, nextHopIp)
		numOfNextHops := len(rEnt.nextHopMap)
		rMgr.logger.Debug("deleted nextHopMap for ", nextHopIp, " numOfNextHops:", numOfNextHops)
		if numOfNextHops == 0 {
			//Remove key from cache
			for idx, val := range rMgr.ipRouteDBKeys {
				if val == ipPrefix {
					rMgr.ipRouteDBKeys = append(rMgr.ipRouteDBKeys[:idx], rMgr.ipRouteDBKeys[idx+1:]...)
					break
				}
			}
			//Delete from route db sw cache
			delete(rMgr.ipRouteDB, ipPrefix)
		}
		for _, plugin := range rMgr.plugins {
			/* Call delete route for asic plugin only if num nexthops = 0*/
/*
			if plugin == retplugin {
				rMgr.logger.Debug("Same plugin type, so dont call delete")
				continue
			}
			rMgr.logger.Err("deleting from plugins in create route clean up")
			if rMgr.pluginRouteDB[pluginRouteDataKey].pluginStatusMap[plugin].status == inactive {
				continue
			}
			if (numOfNextHops == 0 && isPluginAsicDriver(plugin)) || !isPluginAsicDriver(plugin) {
				ipInfo.Op = pluginCommon.PluginOp_Del
				rMgr.logger.Err("deleting from plugins in create route clean up, numOfNextHops:", numOfNextHops)
				ipInfo.RetHdlrFunc = nil
				plugin.AddToRouteChannel(ipInfo)
			}
		}
		if ipInfo.IpType == syscall.AF_INET {
			rMgr.v4RouteCnt -= 1
		} else {
			rMgr.v6RouteCnt -= 1
		}
		if ipInfo.EcmpRoute {
			rMgr.ecmpRouteCnt -= 1
		}
	} else { //case when the current plugin was successful in creating the route
		ipPrefix := ipInfo.RouteDBKeys.(IpPrefix)
		var pluginRouteInfo PluginRouteData
		var exist bool
		newpluginRouteDBEntry := true
		var pluginRouteStatus PluginRouteStatus
		//check if there is a pluginRouteInfo for this prefix
		pluginRouteInfo, exist = rMgr.pluginRouteDB[pluginRouteDataKey]
		if !exist {
			//this is the first plugin call back
			rMgr.logger.Debug("pluginRouteDbEntry not found during create success handler for ipPrefix:", ipPrefix)
			pluginRouteStatusMap := make(map[PluginIntf]PluginRouteStatus)
			//update the routecreated plugin status for this prefix for this plugin
			pluginRouteStatus.status = configured
			pluginRouteStatusMap[retplugin] = pluginRouteStatus
			pluginRouteInfo.pluginStatusMap = pluginRouteStatusMap
			rMgr.pluginRouteDB[pluginRouteDataKey] = pluginRouteInfo

		} else {
			var rEnt IpRouteData
			//other plugins have already returned, check if any of those has fail return codes
			rMgr.logger.Debug("pluginMap for this prefix exists, len(pluginRouteInfo.pluginStatusMap):", len(pluginRouteInfo.pluginStatusMap))
			pluginRouteStatusMap := pluginRouteInfo.pluginStatusMap
			pluginRouteStatus.status = configured
			for _, v := range pluginRouteStatusMap {
				if v.status == inactive {
					//another plugin has failure return code for this prefix route creation
					numOfNextHops := 0
					if rEnt, exist = rMgr.ipRouteDB[ipPrefix]; exist {
						rMgr.logger.Info("ipRouteDBEntry found during create fail cleanup for ipPrefix:", ipPrefix)
						numOfNextHops = len(rEnt.nextHopMap)
					}
					rMgr.logger.Debug(" plugin's status is inactive, numOfNextHops:", numOfNextHops)
					if (numOfNextHops == 0 && isPluginAsicDriver(retplugin)) || !isPluginAsicDriver(retplugin) {
						ipInfo.Op = pluginCommon.PluginOp_Del
						ipInfo.RetHdlrFunc = nil
						retplugin.AddToRouteChannel(ipInfo)
						rMgr.logger.Debug("delete route done for the plugin during create return handler:")
					}
					pluginRouteStatus.status = inactive
					pluginRouteStatusMap[retplugin] = pluginRouteStatus
					pluginRouteInfo.pluginStatusMap = pluginRouteStatusMap
					rMgr.pluginRouteDB[pluginRouteDataKey] = pluginRouteInfo
					return
				}
				newpluginRouteDBEntry = false
			}
			pluginRouteStatusMap[retplugin] = pluginRouteStatus
			pluginRouteInfo.pluginStatusMap = pluginRouteStatusMap
			rMgr.pluginRouteDB[pluginRouteDataKey] = pluginRouteInfo
		}
		if newpluginRouteDBEntry {
			_, routeDbExist := rMgr.ipRouteDB[ipPrefix]
			rMgr.logger.Debug("routeDbExist:", routeDbExist, " pluginroutestatus:", pluginRouteStatus.status)
			if pluginRouteStatus.status == configured {
				//Insert into key cache new keys only
				if !routeDbExist {
					rMgr.ipRouteDBKeys = append(rMgr.ipRouteDBKeys, ipPrefix)
				}
				//Update route db sw cache
				rEnt := ipInfo.RouteEntity.(IpRouteData)
				rMgr.logger.Debug("CreateRouteRetHandler() add rEnt with ecmpgrpid:", rEnt.ecmpGrpId)
				rMgr.ipRouteDB[ipPrefix] = rEnt
				if ipInfo.IpType == syscall.AF_INET {
					rMgr.v4RouteCnt += 1
				} else {
					rMgr.v6RouteCnt += 1
				}
				if ipInfo.EcmpRoute {
					rMgr.ecmpRouteCnt += 1
				}
			}

		}
	}
}
*/
