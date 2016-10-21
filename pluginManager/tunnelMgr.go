package pluginManager

import (
	"asicd/pluginManager/pluginCommon"
	"asicdInt"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"utils/commonDefs"
	"utils/logging"
)

var vtepDb map[string]*vtepStateDbEntry

type vtepStateDbEntry struct {
	IfIndex        int32
	Name           string
	SrcIfName      string
	SrcIfIndex     int32
	Vni            int32
	NextHopIfIndex int32
	NextHopIfName  string
}

type TunnelManager struct {
	plugins       []PluginIntf
	logger        *logging.Writer
	pMgr          *PortManager
	nMgr          *NextHopManager
	iMgr          *IfManager
	rMgr          *RouteManager
	vMgr          *VlanManager
	vxlanEnabled  map[int]int
	initComplete  bool
	IfIdMap       map[int][]int
	IfIdCurr      map[int]int
	notifyChannel chan<- []byte
	PortRefCnt    map[int32]int
	vxlanCfgDb    map[int32]*asicdInt.Vxlan
	vtepCfgDb     map[string]*asicdInt.Vtep
}

// L3intf db
var TunnelfMgr TunnelManager

func (tMgr *TunnelManager) Init(rsrcMgrs *ResourceManagers, pluginList []PluginIntf, logger *logging.Writer, notifyChannel chan<- []byte) {
	tMgr.pMgr = rsrcMgrs.PortManager
	tMgr.iMgr = rsrcMgrs.IfManager
	tMgr.nMgr = rsrcMgrs.NextHopManager
	tMgr.rMgr = rsrcMgrs.RouteManager
	tMgr.vMgr = rsrcMgrs.VlanManager
	tMgr.notifyChannel = notifyChannel
	tMgr.logger = logger

	if tMgr.vxlanEnabled == nil {
		tMgr.vxlanEnabled = make(map[int]int)
	}

	for _, p := range pluginList {
		tMgr.plugins = append(tMgr.plugins, p)
	}

	tMgr.vxlanCfgDb = make(map[int32]*asicdInt.Vxlan)
	tMgr.vtepCfgDb = make(map[string]*asicdInt.Vtep)
}

func (tMgr *TunnelManager) Deinit() {
	// TODO
	for _, config := range tMgr.vtepCfgDb {
		tMgr.DeleteVtepIntf(config)
	}

	for _, config := range tMgr.vxlanCfgDb {
		tMgr.DeleteVxlan(config)
	}
}

func (tMgr *TunnelManager) CreateVxlan(config *asicdInt.Vxlan) (bool, error) {

	vMgr := tMgr.vMgr

	//if (config.AdminState != INTF_STATE_UP) && (config.AdminState != INTF_STATE_DOWN) {
	//	return -1, false, errors.New("Invalid AdminState value specified during vxlan create")
	//}
	var vlanId int = int(config.VlanId)
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return false, errors.New("Invalid vlan id specified during vxlan create)")
	}
	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(config.IntfRefList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list vxlan create")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(config.UntagIntfRefList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list vxlan create")
	}

	tMgr.vxlanCfgDb[config.Vni] = config

	tMgr.vxlanCfgDb[config.Vni] = config
	for _, plugin := range tMgr.plugins {
		plugin.CreateVxlan(config)
		plugin.AddInterfaceToVxlanBridge(uint32(config.Vni), ifIndexList, untagIfIndexList)
	}
	return true, nil
}
func (tMgr *TunnelManager) DeleteVxlan(config *asicdInt.Vxlan) (bool, error) {
	vMgr := tMgr.vMgr

	var vlanId int = int(config.VlanId)
	if (vlanId < pluginCommon.SYS_RSVD_VLAN) || (vlanId > pluginCommon.MAX_VLAN_ID) ||
		((vlanId > vMgr.sysRsvdVlanMin) && (vlanId < vMgr.sysRsvdVlanMax)) {
		return false, errors.New("Invalid vlan id specified during vlan delete")
	}
	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(config.IntfRefList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(config.UntagIntfRefList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list")
	}

	for _, plugin := range tMgr.plugins {
		plugin.DelInterfaceFromVxlanBridge(uint32(config.Vni), ifIndexList, untagIfIndexList)
		plugin.DeleteVxlan(config)
	}
	return true, nil
}

func (tMgr *TunnelManager) AddHostInterfaceToVxlan(Vni int32, IntfRefList, UntagIntfRefList []string) (bool, error) {
	vMgr := tMgr.vMgr

	vxlan, ok := tMgr.vxlanCfgDb[Vni]
	if !ok {
		return false, errors.New(fmt.Sprintln("Failed to find Vni", Vni, "unable to add host interface", IntfRefList, UntagIntfRefList))
	}

	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(IntfRefList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list vxlan create")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(UntagIntfRefList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list vxlan create")
	}

	newifindexlist := make([]int32, 0)
	newuntagifindexlist := make([]int32, 0)
	for idx, newintfref := range IntfRefList {
		foundref := false
		for _, intfref := range vxlan.IntfRefList {
			if newintfref == intfref {
				foundref = true
				break
			}
		}
		if !foundref {
			tMgr.vxlanCfgDb[Vni].IntfRefList = append(IntfRefList, newintfref)
			newifindexlist = append(newifindexlist, ifIndexList[idx])
		}
	}
	for idx, newintfref := range UntagIntfRefList {
		foundref := false
		for _, intfref := range vxlan.UntagIntfRefList {
			if newintfref == intfref {
				foundref = true
				break
			}
		}
		if !foundref {
			tMgr.vxlanCfgDb[Vni].UntagIntfRefList = append(tMgr.vxlanCfgDb[Vni].UntagIntfRefList, newintfref)
			newuntagifindexlist = append(newuntagifindexlist, untagIfIndexList[idx])
		}
	}
	if len(newifindexlist) > 0 || len(newuntagifindexlist) > 0 {
		for _, plugin := range tMgr.plugins {

			plugin.AddInterfaceToVxlanBridge(uint32(Vni), newifindexlist, newuntagifindexlist)
		}
	}
	return true, nil
}

func (tMgr *TunnelManager) DelFromHostInterfaceFromVxlan(Vni int32, IntfRefList, UntagIntfRefList []string) (bool, error) {
	vMgr := tMgr.vMgr

	vxlan, ok := tMgr.vxlanCfgDb[Vni]
	if !ok {
		return false, errors.New(fmt.Sprintln("Failed to find Vni", Vni, "unable to del host interface", IntfRefList, UntagIntfRefList))
	}

	ifIndexList, err := vMgr.parseIntfStrListToIfIndexList(IntfRefList)
	if err != nil {
		return false, errors.New("Failed to map intf list string to ifindex list vxlan create")
	}
	untagIfIndexList, err := vMgr.parseIntfStrListToIfIndexList(UntagIntfRefList)
	if err != nil {
		return false, errors.New("Failed to map untagged intf list string to ifindex list vxlan create")
	}

	newifindexlist := make([]int32, 0)
	newuntagifindexlist := make([]int32, 0)
	for idx, newintfref := range IntfRefList {
		foundref := false
		for _, intfref := range vxlan.IntfRefList {
			if newintfref == intfref {
				foundref = true
				break
			}
		}
		if foundref {
			tMgr.vxlanCfgDb[Vni].IntfRefList = append(tMgr.vxlanCfgDb[Vni].IntfRefList[:idx], tMgr.vxlanCfgDb[Vni].IntfRefList[idx+1:]...)
			newifindexlist = append(newifindexlist, ifIndexList[idx])
		}
	}
	for idx, newintfref := range UntagIntfRefList {
		foundref := false
		for _, intfref := range vxlan.UntagIntfRefList {
			if newintfref == intfref {
				foundref = true
				break
			}
		}
		if foundref {
			tMgr.vxlanCfgDb[Vni].UntagIntfRefList = append(tMgr.vxlanCfgDb[Vni].UntagIntfRefList[:idx], tMgr.vxlanCfgDb[Vni].UntagIntfRefList[idx+1:]...)
			newuntagifindexlist = append(newuntagifindexlist, untagIfIndexList[idx])
		}
	}
	if len(newifindexlist) > 0 || len(newuntagifindexlist) > 0 {
		for _, plugin := range tMgr.plugins {
			plugin.DelInterfaceFromVxlanBridge(uint32(Vni), newifindexlist, newuntagifindexlist)
		}
	}
	return true, nil
}

func (tMgr *TunnelManager) CreateVtepIntf(config *asicdInt.Vtep) (bool, error) {

	tMgr.vtepCfgDb[config.IfName] = config

	id := tMgr.iMgr.AllocateIfIndex(config.IfName, 0, commonDefs.IfTypeVtep)

	dip := net.ParseIP(config.DstIp)
	dipBytes := dip.To4()
	dip32 := binary.BigEndian.Uint32(dipBytes)
	//dip32 := uint32(dipBytes[0]<<24) | uint32(dipBytes[1]<<16) | uint32(dipBytes[2]<<8) | uint32(dipBytes[3])

	sip := net.ParseIP(config.SrcIp)
	sipBytes := sip.To4()
	sip32 := binary.BigEndian.Uint32(sipBytes)
	netMac, _ := net.ParseMAC(config.SrcMac)

	//route, _ := tMgr.rMgr.GetIPRoute(config.DstIp)

	// we know this exists cause vtep create would not have happend otherwise
	nhip := net.ParseIP(config.NextHopIp)
	nhipBytes := nhip.To4()
	nhip32 := binary.BigEndian.Uint32(nhipBytes)
	nextHopId := tMgr.nMgr.GetNextHopIdForIPv4Addr(nhip32, commonDefs.IfTypePort)

	for _, plugin := range tMgr.plugins {

		// keep track of enabling port specific settings that should be set
		// for vxlan
		if isPluginAsicDriver(plugin) {
			if _, ok := tMgr.vxlanEnabled[int(config.SrcIfIndex)]; !ok {
				tMgr.vxlanEnabled[int(config.NextHopIfIndex)] = 0
			}
			if tMgr.vxlanEnabled[int(config.NextHopIfIndex)] == 0 {
				// enable vxlan settings on the port
				tMgr.vxlanEnabled[int(config.NextHopIfIndex)] = 1
				plugin.VxlanPortEnable(int32(config.NextHopIfIndex))
			} else {
				tMgr.vxlanEnabled[int(config.NextHopIfIndex)] = 1
			}
		}
		tMgr.logger.Debug(fmt.Sprintf("CreateVtepIntf: id %d, %#v, %#v", id, config, plugin))

		plugin.CreateVtepIntf(config.IfName,
			int(config.SrcIfIndex),
			int(config.NextHopIfIndex),
			int32(config.Vni),
			dip32,
			sip32,
			netMac,
			int32(config.VlanId),
			int32(config.TTL),
			int32(config.UDP),
			uint64(nextHopId))
		//tMgr.logger.Info(fmt.Sprintf("CreateVtepIntf: %d", rv))
		//if rv < 0 {
		//	return false, errors.New("Failed to update port configuration")
		//}
	}

	// get the ifindex of the src interface port

	if vtepDb == nil {
		vtepDb = make(map[string]*vtepStateDbEntry)
	}

	vtepDb[config.IfName] = &vtepStateDbEntry{
		Vni:            config.Vni,
		Name:           config.IfName,
		IfIndex:        int32(id),
		SrcIfIndex:     config.SrcIfIndex,
		SrcIfName:      config.SrcIfName,
		NextHopIfIndex: config.NextHopIfIndex,
		NextHopIfName:  config.NextHopIfName,
	}

	//Publish notification - ARPd listens to vtep create notification
	msg := pluginCommon.VtepNotifyMsg{
		Vni:        config.Vni,
		VtepName:   config.IfName,
		IfIndex:    int32(id),
		SrcIfIndex: config.SrcIfIndex,
		SrcIfName:  config.SrcIfName,
	}
	msgBuf, err := json.Marshal(msg)
	if err != nil {
		tMgr.logger.Err("Failed to marshal vlan create message")
	}
	notification := pluginCommon.AsicdNotification{
		MsgType: uint8(pluginCommon.NOTIFY_VTEP_CREATE),
		Msg:     msgBuf,
	}
	notificationBuf, err := json.Marshal(notification)
	if err != nil {
		tMgr.logger.Err("Failed to marshal vlan create message")
	}
	tMgr.notifyChannel <- notificationBuf
	return true, nil
}
func (tMgr *TunnelManager) DeleteVtepIntf(config *asicdInt.Vtep) (bool, error) {

	dip := net.ParseIP(config.DstIp)
	dipBytes := dip.To4()
	//dip32 := uint32(dipBytes[3]<<3) | uint32(dipBytes[2]<<2) | uint32(dipBytes[1]<<1) | uint32(dipBytes[0])
	dip32 := binary.BigEndian.Uint32(dipBytes)

	sip := net.ParseIP(config.SrcIp)
	sipBytes := sip.To4()
	sip32 := binary.BigEndian.Uint32(sipBytes)
	//sip32 := uint32(sipBytes[3]<<3) | uint32(sipBytes[2]<<2) | uint32(sipBytes[1]<<1) | uint32(sipBytes[0])
	netMac, _ := net.ParseMAC(config.SrcMac)

	for _, plugin := range tMgr.plugins {

		plugin.DeleteVtepIntf(config.IfName, int(config.SrcIfIndex), int(config.NextHopIfIndex),
			int32(config.Vni), dip32, sip32, netMac, int32(config.VlanId),
			int32(config.TTL), int32(config.UDP))

		// keep track of disabling port specific settings that should be cleared
		// for vxlan
		if isPluginAsicDriver(plugin) {
			if _, ok := tMgr.vxlanEnabled[int(config.SrcIfIndex)]; ok {
				if tMgr.vxlanEnabled[int(config.SrcIfIndex)] == 1 {
					// enable vxlan settings on the port
					delete(tMgr.vxlanEnabled, int(config.SrcIfIndex))
					plugin.VxlanPortDisable(int32(config.SrcIfIndex))
				} else {
					tMgr.vxlanEnabled[int(config.SrcIfIndex)]--
				}
			}
		}

		//tMgr.logger.Info(fmt.Sprintf("DeleteVtepIntf: %d", rv))
		//if rv < 0 {
		//	return false, errors.New("Failed to update port configuration")
		//}
	}
	// get the ifindex of the src interface port
	if state, ok := vtepDb[config.IfName]; ok {

		//Publish notification - ARPd listens to vlan create notification
		msg := pluginCommon.VtepNotifyMsg{
			Vni:        state.Vni,
			VtepName:   state.Name,
			IfIndex:    state.IfIndex,
			SrcIfIndex: state.SrcIfIndex,
			SrcIfName:  state.SrcIfName,
		}
		msgBuf, err := json.Marshal(msg)
		if err != nil {
			tMgr.logger.Err("Failed to marshal vlan create message")
		}
		notification := pluginCommon.AsicdNotification{
			MsgType: uint8(pluginCommon.NOTIFY_VTEP_DELETE),
			Msg:     msgBuf,
		}
		notificationBuf, err := json.Marshal(notification)
		if err != nil {
			tMgr.logger.Err("Failed to marshal vlan create message")
		}
		tMgr.notifyChannel <- notificationBuf

		delete(vtepDb, config.IfName)
	}
	return true, nil
}

func (tMgr *TunnelManager) LearnFdbVtep(mac string, name string, ifIndex int32) {
	for _, plugin := range tMgr.plugins {
		if !isPluginAsicDriver(plugin) {
			netMac, _ := net.ParseMAC(mac)
			plugin.LearnFdbVtep(netMac, name, ifIndex)
		}
		//tMgr.logger.Info(fmt.Sprintf("DeleteVtepIntf: %d", rv))
		//if rv < 0 {
		//	return false, errors.New("Failed to update port configuration")
		//}
	}
}
