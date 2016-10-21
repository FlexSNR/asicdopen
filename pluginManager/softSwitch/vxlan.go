// vxlan_linux.go
// NOTE: this is meant for testing, it should eventually live in asicd
package softSwitch

import (
	"asicd/pluginManager/pluginCommon"
	"asicdInt"
	"fmt"
	"net"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	//"os/exec"
)

var VxlanConfigMode string = "proxy"

var VxlanDB map[uint32]*VxlanDbEntry

// TOOD convert this to petricia trie
var macDb map[int32][]*VxlanMacDbEntry

type VxlanMacDbEntry struct {
	vtepName string
	mac      net.HardwareAddr
}

type VxlanDbEntry struct {
	Vni    uint32
	VlanId uint16 // used to tag inner ethernet frame when egressing
	Group  net.IP // multicast group IP
	MTU    uint32 // MTU size for each VTEP
	Brg    *netlink.Bridge
	Links  []*netlink.Link
}

// bridge for the VNI
type VxlanConfig struct {
	Vni    uint32
	VlanId uint16 // used to tag inner ethernet frame when egressing
	Group  net.IP // multicast group IP
	MTU    uint32 // MTU size for each VTEP
}

// tunnel endpoint for the VxLAN
type VtepConfig struct {
	VtepId                uint32           `SNAPROUTE: KEY` //VTEP ID.
	Vni                   uint32           // Vxlan id reference
	VtepName              string           //VTEP instance name.
	SrcIfName             string           //Source interface ifIndex.
	UDP                   uint16           //vxlan udp port.  Deafult is the iana default udp port
	TTL                   uint16           //TTL of the Vxlan tunnel
	TOS                   uint16           //Type of Service
	InnerVlanHandlingMode int32            //The inner vlan tag handling mode.
	Learning              bool             //specifies if unknown source link layer  addresses and IP addresses are entered into the VXLAN  device forwarding database.
	Rsc                   bool             //specifies if route short circuit is turned on.
	L2miss                bool             //specifies if netlink LLADDR miss notifications are generated.
	L3miss                bool             //specifies if netlink IP ADDR miss notifications are generated.
	SrcIp                 net.IP           //Source IP address for the static VxLAN tunnel
	DstIp                 net.IP           //Destination IP address for the static VxLAN tunnel
	VlanId                uint16           //Vlan Id to encapsulate with the vtep tunnel ethernet header
	SrcMac                net.HardwareAddr //Src Mac assigned to the VTEP within this VxLAN. If an address is not assigned the the local switch address will be used.
	//TunnelDstMac          net.HardwareAddr
}

func ConvertThriftVxlanToVxlanConfig(c *asicdInt.Vxlan) (*VxlanConfig, error) {

	return &VxlanConfig{
		Vni:    uint32(c.Vni),
		VlanId: uint16(c.VlanId),
		Group:  net.ParseIP(c.McDestIp),
		MTU:    uint32(c.Mtu),
	}, nil
}

func ConvertThriftVtepToVtepConfig(c *asicdInt.Vtep) (*VtepConfig, error) {

	var mac net.HardwareAddr
	var ip net.IP
	if c.SrcMac != "" {
		mac, _ = net.ParseMAC(c.SrcMac)
	}
	if c.SrcIp != "" {
		ip = net.ParseIP(c.SrcIp)
	}

	srcName := "eth2" // asicDGetLinuxIfName(c.SrcIfIndex)

	//logger.Info(fmt.Sprintf("Forcing Vtep %s to use Lb %s SrcMac %s Ip %s", c.VtepName, name, mac, ip))
	return &VtepConfig{
		Vni:       uint32(c.Vni),
		VtepName:  string(c.IfName),
		SrcIfName: srcName,
		UDP:       uint16(c.UDP),
		TTL:       uint16(c.TTL),
		//TOS:       uint16(c.TOS),
		//InnerVlanHandlingMode: c.InnerVlanHandlingMode,
		Learning: c.Learning,
		//Rsc:          c.Rsc,
		//L2miss:       c.L2miss,
		//L3miss:       c.L3miss,
		SrcIp:  ip,
		DstIp:  net.ParseIP(c.DstIp),
		VlanId: uint16(c.VlanId),
		SrcMac: mac,
	}, nil
}

// createVxLAN is the equivalent to creating a bridge in the linux
// The VNI is actually associated with the VTEP so lets just create a bridge
// if necessary
func (v *SoftSwitchPlugin) CreateVxlan(config *asicdInt.Vxlan) {

	c, _ := ConvertThriftVxlanToVxlanConfig(config)

	v.logger.Info("Calling softswitch CreateVxlan")
	if _, ok := VxlanDB[c.Vni]; !ok {
		VxlanDB[c.Vni] = &VxlanDbEntry{
			Vni:    c.Vni,
			VlanId: c.VlanId,
			Group:  c.Group,
			MTU:    c.MTU,
			Links:  make([]*netlink.Link, 0),
		}
		// lets create a bridge if it does not exists
		// bridge should be based on the VLAN used by a
		// customer.
		brname := fmt.Sprintf("%s%d", pluginCommon.VXLAN_VXLAN_PREFIX, c.VlanId)
		bridge := &netlink.Bridge{
			LinkAttrs: netlink.LinkAttrs{
				Name: brname,
				MTU:  int(c.MTU),
			},
		}

		if err := netlink.LinkAdd(bridge); err != nil {
			v.logger.Err(err.Error())
		}

		link, err := netlink.LinkByName(bridge.Attrs().Name)
		if err != nil {
			v.logger.Err(err.Error())
		}

		vxlanDbEntry := VxlanDB[c.Vni]
		vxlanDbEntry.Brg = link.(*netlink.Bridge)
		VxlanDB[c.Vni] = vxlanDbEntry
		// lets set the vtep interface to up
		if err := netlink.LinkSetUp(bridge); err != nil {
			v.logger.Err(err.Error())
		}
	}
}

func (v *SoftSwitchPlugin) DeleteVxlan(config *asicdInt.Vxlan) {

	c, _ := ConvertThriftVxlanToVxlanConfig(config)

	if vxlan, ok := VxlanDB[c.Vni]; ok {
		for i, link := range vxlan.Links {
			// lets set the vtep interface to up
			if err := netlink.LinkSetDown(*link); err != nil {
				v.logger.Err(err.Error())
			}
			if err := netlink.LinkDel(*link); err != nil {
				v.logger.Err(err.Error())
			}

			vxlanDbEntry := VxlanDB[c.Vni]
			vxlanDbEntry.Links = append(vxlanDbEntry.Links[:i], vxlanDbEntry.Links[i+1:]...)
			VxlanDB[c.Vni] = vxlanDbEntry
		}

		link, err := netlink.LinkByName(vxlan.Brg.Name)
		if err != nil {
			v.logger.Err(err.Error())
		}

		// lets set the vtep interface to up
		if err := netlink.LinkSetDown(link); err != nil {
			v.logger.Err(err.Error())
		}
		if err := netlink.LinkDel(link); err != nil {
			v.logger.Err(err.Error())
		}

		delete(VxlanDB, c.Vni)
	}
}

func (v SoftSwitchPlugin) AddInterfaceToVxlanBridge(vni uint32, ifIndexList, untagIfIndexList []int32) {

	if vxlan, ok := VxlanDB[vni]; ok {
		AddInterfacesToBridge(int(vxlan.VlanId), ifIndexList, untagIfIndexList, vxlan.Brg.Name, vxlan.Brg)
	} else {
		v.logger.Err("Error Unabled to add interface", ifIndexList, untagIfIndexList, " to Vni", vni, "because does not exist")
	}
}

func (v SoftSwitchPlugin) DelInterfaceFromVxlanBridge(vni uint32, ifIndexList, untagIfIndexList []int32) {

	if vxlan, ok := VxlanDB[vni]; ok {
		DeleteInterfacesFromBridge(int(vxlan.VlanId), ifIndexList, untagIfIndexList)
	} else {
		v.logger.Err("Error Unabled to del interface", ifIndexList, untagIfIndexList, " from Vni", vni, "because does not exist")
	}
}

func (v *SoftSwitchPlugin) CreateVtepIntf(vtepIfName string, ifIndex int, nextHopIfIndex int, vni int32, dstIpAddr uint32, srcIpAddr uint32, macAddr net.HardwareAddr, VlanId int32, ttl int32, udp int32, nextHopIndex uint64) {

	v.logger.Info("CreatingVtepIntf start ", VxlanConfigMode)
	VtepIfName := vtepIfName

	if VxlanConfigMode == "linux" {
		// assumes physical port
		ifName := getName(int32(ifIndex))

		link, err := netlink.LinkByName(ifName)
		if err != nil {
			v.logger.Err(fmt.Sprintf("Error finding link %s: %s", ifName, err.Error()))
			return
		}
		/* 4/6/16 DID Not work, packets were never received on VTEP */
		vtep := &netlink.Vxlan{
			LinkAttrs: netlink.LinkAttrs{
				Name: VtepIfName,
				//MasterIndex: VxlanDB[c.VxlanId].Brg.Attrs().Index,
				HardwareAddr: macAddr,
				//MTU:          VxlanDB[c.VxlanId].Brg.Attrs().MTU,
				MTU: 1550,
			},
			VxlanId:      int(vni),
			VtepDevIndex: link.Attrs().Index,
			SrcAddr:      net.ParseIP(fmt.Sprintf("%d.%d.%d.%d", (srcIpAddr>>24)&0xff, (srcIpAddr>>16)&0xff, (srcIpAddr>>8)&0xff, (srcIpAddr>>0)&0xff)),
			Group:        net.ParseIP(fmt.Sprintf("%d.%d.%d.%d", (dstIpAddr>>24)&0xff, (dstIpAddr>>16)&0xff, (dstIpAddr>>8)&0xff, (dstIpAddr>>0)&0xff)),
			TTL:          int(ttl),
			Age:          300,
			Port:         int(nl.Swap16(uint16(udp))),
		}
		//equivalent to linux command:
		// ip link add DEVICE type vxlan id ID [ dev PHYS_DEV  ] [ { group
		//         | remote } IPADDR ] [ local IPADDR ] [ ttl TTL ] [ tos TOS ] [
		//          port MIN MAX ] [ [no]learning ] [ [no]proxy ] [ [no]rsc ] [
		//          [no]l2miss ] [ [no]l3miss ]
		if err := netlink.LinkAdd(vtep); err != nil {
			v.logger.Err(err.Error())
		}

	} else {

		// Veth will create two interfaces
		// VtepName and VtepName + Int
		// the VtepNam + Int interface will be used by Vxland to rx packets
		// from other daemons and to send packets received from physical port
		// to the daemons
		//
		//  physical port <--> vxland (if vxlan packet) <--> vtepName Int <-->
		//  vtepName <--> Other Daemons listening
		//  on this vtepName interface
		vtep := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{
				Name:         VtepIfName,
				MasterIndex:  VxlanDB[uint32(vni)].Brg.Attrs().Index,
				HardwareAddr: macAddr,
				MTU:          VxlanDB[uint32(vni)].Brg.Attrs().MTU,
			},
			PeerName: VtepIfName + "Int",
		}
		v.logger.Info(fmt.Sprintln("Creating veth ", vtep))
		//equivalent to linux command:
		// ip link add DEVICE type vxlan id ID [ dev PHYS_DEV  ] [ { group
		//         | remote } IPADDR ] [ local IPADDR ] [ ttl TTL ] [ tos TOS ] [
		//          port MIN MAX ] [ [no]learning ] [ [no]proxy ] [ [no]rsc ] [
		//          [no]l2miss ] [ [no]l3miss ]
		if err := netlink.LinkAdd(vtep); err != nil {
			v.logger.Err(err.Error())
		}

		link, err := netlink.LinkByName(VtepIfName + "Int")
		if err != nil {
			v.logger.Err(fmt.Sprintf("Link by Name vtep:", err.Error()))
		}

		/* ON RECREATE - Link up is failing with reason:
		   transport endpoint is not connected lets delay
		   till it is connected */
		// lets set the vtep interface to up
		for i := 0; i < 10; i++ {
			err := netlink.LinkSetUp(link)
			if err != nil && i < 10 {
				v.logger.Info(fmt.Sprintf("createVtep: %s link not connected yet waiting 5ms", VtepIfName))
				time.Sleep(time.Millisecond * 5)
			} else if err != nil {
				v.logger.Err(err.Error())
			} else {
				break
			}
		}
	}

	link, err := netlink.LinkByName(VtepIfName)
	if err != nil {
		v.logger.Err(fmt.Sprintf("Link by Name vtep:", err.Error()))
	}

	// found that hte mac we are trying to set fails lets try and add it again
	if err := netlink.LinkSetHardwareAddr(link, macAddr); err != err {
		v.logger.Err(fmt.Sprintf("LinkSetHardwareAddr vtep:", err.Error()))
	}

	// equivalent to linux command:
	/* bridge fdb add - add a new fdb entry
	       This command creates a new fdb entry.

	       LLADDR the Ethernet MAC address.

	       dev DEV
	              the interface to which this address is associated.

	              self - the address is associated with a software fdb (default)

	              embedded - the address is associated with an offloaded fdb

	              router - the destination address is associated with a router.
	              Valid if the referenced device is a VXLAN type device and has
	              route shortcircuit enabled.

	      The next command line parameters apply only when the specified device
	      DEV is of type VXLAN.

	       dst IPADDR
	              the IP address of the destination VXLAN tunnel endpoint where
	              the Ethernet MAC ADDRESS resides.

	       vni VNI
	              the VXLAN VNI Network Identifier (or VXLAN Segment ID) to use to
	              connect to the remote VXLAN tunnel endpoint.  If omitted the
	              value specified at vxlan device creation will be used.

	       port PORT
	              the UDP destination PORT number to use to connect to the remote
	              VXLAN tunnel endpoint.  If omitted the default value is used.

	       via DEVICE
	              device name of the outgoing interface for the VXLAN device
	              driver to reach the remote VXLAN tunnel endpoint.


			// values taken from linux/neighbour.h
	*/
	/* TAKEN CARE OF BY ARP
	if VxlanConfigMode == "linux" {

		if c.TunnelDstIp != nil &&
			c.TunnelDstMac != nil {
			neigh := &netlink.Neigh{
				LinkIndex: link.Attrs().Index,
				//Family:       netlink.NDA_VNI,                           // NDA_VNI
				State:        netlink.NUD_NOARP | netlink.NUD_PERMANENT, // NUD_NOARP (0x40) | NUD_PERMANENT (0x80)
				Type:         1,
				Flags:        netlink.NTF_SELF, // NTF_SELF
				IP:           c.TunnelDstIp,
				HardwareAddr: c.TunnelDstMac,
			}
			v.logger.Info(fmt.Sprintf("neighbor: %#v", neigh))
			if err := netlink.NeighSet(neigh); err != nil {
				v.logger.Err(fmt.Sprintf("NeighSet:", err.Error()))
			}
		retry_neighbor_set:
			neighList, err := netlink.NeighList(neigh.LinkIndex, neigh.Family)
			if err == nil {

				for _, n := range neighList {
					foundNeighbor := false
					if len(neigh.IP) == len(n.IP) {
						for i, _ := range neigh.IP {
							if neigh.IP[i] == n.IP[i] {
								foundNeighbor = true
							} else {
								foundNeighbor = false
							}
						}
					}
					if foundNeighbor {
						v.logger.Info("Found Neighbor ip")
						if n.State == netlink.NUD_FAILED {
							v.logger.Info(fmt.Sprintf("retry neighbor: %#v", neigh))
							if err := netlink.NeighSet(neigh); err != nil {
								v.logger.Err(fmt.Sprintf("NeighSet:", err.Error()))
								goto retry_neighbor_set
							}
						}
					}
				}
			}
		} else {
			v.logger.Info(fmt.Sprintf("neighbor: not configured dstIp %#v dstmac %#v", c.TunnelDstIp, c.TunnelDstMac))
		}
	}
	*/
	vxlanDbEntry := VxlanDB[uint32(vni)]
	vxlanDbEntry.Links = append(vxlanDbEntry.Links, &link)
	VxlanDB[uint32(vni)] = vxlanDbEntry

	if err := netlink.LinkSetMaster(link, vxlanDbEntry.Brg); err != nil {
		v.logger.Err(err.Error())
	}

	/* ON RECREATE - Link up is failing with reason:
	   transport endpoint is not connected lets delay
	   till it is connected */
	// lets set the vtep interface to up
	for i := 0; i < 10; i++ {
		err := netlink.LinkSetUp(link)
		if err != nil && i < 10 {
			v.logger.Info(fmt.Sprintf("createVtep: %s link not connected yet waiting 5ms", VtepIfName))
			time.Sleep(time.Millisecond * 5)
		} else if err != nil {
			v.logger.Err(err.Error())
		} else {
			break
		}
	}
}

func (v *SoftSwitchPlugin) DeleteVtepIntf(vtepIfName string, ifIndex int, nextHopIfIndex int, vni int32,
	dstIpAddr uint32, srcIpAddr uint32, macAddr net.HardwareAddr,
	VlanId int32, ttl int32, udp int32) {
	foundEntry := false
	VtepIfName := vtepIfName
	if vxlanentry, ok := VxlanDB[uint32(vni)]; ok {
		for i, link := range vxlanentry.Links {
			var linkName string
			if VxlanConfigMode == "linux" {
				linkName = (*link).(*netlink.Vxlan).Attrs().Name
			} else {
				linkName = (*link).(*netlink.Veth).Attrs().Name
			}
			if linkName == VtepIfName {
				v.logger.Info(fmt.Sprintf("deleteVtep: link found %s looking for %s", linkName, VtepIfName))
				foundEntry = true
				vxlanDbEntry := VxlanDB[uint32(vni)]
				vxlanDbEntry.Links = append(vxlanDbEntry.Links[:i], vxlanDbEntry.Links[i+1:]...)
				VxlanDB[uint32(vni)] = vxlanDbEntry
				break
			}
		}
	}

	if foundEntry {
		link, err := netlink.LinkByName(VtepIfName)
		if err != nil {
			v.logger.Err(err.Error())
		}
		if err := netlink.LinkSetDown(link); err != nil {
			v.logger.Err(err.Error())
		}

		if err := netlink.LinkDel(link); err != nil {
			v.logger.Err(err.Error())
		}
	} else {
		v.logger.Err("Unable to find vtep in vxlan db")
	}
}

func (v *SoftSwitchPlugin) LearnFdbVtep(mac net.HardwareAddr, vtepname string, ifindex int32) {

	if macDb == nil {
		macDb = make(map[int32][]*VxlanMacDbEntry, 0)
	}

	if macList, ok := macDb[ifindex]; ok {
		for _, macentry := range macList {
			if macentry.mac.String() == mac.String() {
				return
			}
		}
		macDb[ifindex] = append(macDb[ifindex], &VxlanMacDbEntry{mac: mac,
			vtepName: vtepname})
		link, _ := netlink.LinkByName(vtepname)
		if link != nil {
			// create fdb entry
			neigh := &netlink.Neigh{
				LinkIndex: link.Attrs().Index,
				//Family:       netlink.NDA_VNI,                           // NDA_VNI
				State:        netlink.NUD_NOARP, // NUD_NOARP (0x40) | NUD_PERMANENT (0x80)
				Type:         1,
				Flags:        netlink.NTF_SELF, // NTF_SELF
				HardwareAddr: mac,
			}
			if err := netlink.NeighAppend(neigh); err != nil {
				v.logger.Err(fmt.Sprintf("NeighSet:", err.Error()))
			}
		}
	}
}

func (v *SoftSwitchPlugin) VxlanPortEnable(portNum int32) {

}
func (v *SoftSwitchPlugin) VxlanPortDisable(portNum int32) {

}
