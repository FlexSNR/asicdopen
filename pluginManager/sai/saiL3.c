#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <syslog.h>

#include <sai.h>
#include <saistatus.h>
#include <saitypes.h>

#include "pluginCommon.h"

sai_virtual_router_api_t* sai_vr_api_tbl = NULL;
sai_route_api_t* sai_route_api_tbl = NULL;
sai_router_interface_api_t* sai_rif_api_tbl = NULL;
sai_neighbor_api_t* sai_nbr_api_tbl = NULL;
sai_next_hop_api_t* sai_nh_api_tbl = NULL;
sai_next_hop_group_api_t* sai_nh_grp_api_tbl = NULL;

/* Global VR object id */
sai_object_id_t globalVrId;

/* Router interface object id's */
sai_object_id_t vlan_rif_oid[MAX_VLAN_ID-1];
sai_object_id_t port_rif_oid[MAX_NUM_PORTS];
sai_object_id_t loopback_rif_oid[MAX_LOOPBACK_INTFS];

#ifdef SAI_BUILD
extern sai_mac_t switchMacAddr;
#endif

/* Add interface route */
static int SaiAddInterfaceRoute(const uint32_t ipAddr)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_attribute_t route_attr = {0};
    sai_unicast_route_entry_t uc_route_entry = {0};

    uc_route_entry.vr_id = globalVrId;
    uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
    uc_route_entry.destination.addr.ip4 = htonl(ipAddr);
    uc_route_entry.destination.mask.ip4 = htonl(0xFFFFFFFF);
    route_attr.id = SAI_ROUTE_ATTR_PACKET_ACTION;
    route_attr.value.s32 = SAI_PACKET_ACTION_TRAP;

    syslog(LOG_INFO, "Creating interface ipv4 route to cpu for %x", ipAddr);
    rv = sai_route_api_tbl->create_route(&uc_route_entry, 1, &route_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to add interface route to cpu for %x. Error : %llu",
                ipAddr, (long long unsigned int)rv);
        return -1;
    }
#endif
    return 0;
}

int SaiRouteAddDel(uint32_t *ipAddr, int ip_type, bool add)
{
    sai_status_t rv;
    int i = 0, j = 0;
    sai_attribute_t route_attr = {0}, get_rttar = {0};
    sai_unicast_route_entry_t uc_route_entry = {0};

    uc_route_entry.vr_id = globalVrId;
    route_attr.id = SAI_ROUTE_ATTR_PACKET_ACTION;
    route_attr.value.s32 = SAI_PACKET_ACTION_TRAP;
    syslog(LOG_INFO, "Route Attributes Getting set are id: %d and value: 0x%x\n", route_attr.id, route_attr.value.s32);
    switch (ip_type) {
        case IP_TYPE_IPV4:
            uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
            uc_route_entry.destination.addr.ip4 = htonl(ipAddr[0]);
            uc_route_entry.destination.mask.ip4 = htonl(0xFFFFFFFF);
            break;

        case IP_TYPE_IPV6:
            uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV6;
            for (i = 0; i < 4; i++) {
                uint32_t ip = htonl(ipAddr[i]);
                int k = 0;
                while (k < 4) {
                    uc_route_entry.destination.addr.ip6[j] = ip >> (k*8);
                    k++;
                    j++;
                }
            }
            memset(&uc_route_entry.destination.mask.ip6, 0xFF, sizeof(uc_route_entry.destination.mask.ip6));
            break;
    }
    switch (add) {
        case true:
            syslog(LOG_INFO, "Creating interface ip route to cpu");
            rv = sai_route_api_tbl->create_route(&uc_route_entry, 1, &route_attr);
            break;
        case false:
            syslog(LOG_INFO, "Deleting interface ip route to cpu");
            rv = sai_route_api_tbl->remove_route(&uc_route_entry);
            break;
    }
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to add interface route to cpu. Error : %llu", (long long unsigned int)rv);
        return -1;
    }
    return 0;
}

int SaiCreateIPIntf(uint32_t *ipAddr, int maskLen, int vlanId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    int attrIdx = 0;
    sai_attribute_t rif_attr[4];

    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_VIRTUAL_ROUTER_ID; //entry 0
    rif_attr[attrIdx].value.oid = globalVrId;
    attrIdx++; // attribute 1
    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_TYPE; // entry 1
    rif_attr[attrIdx].value.s32 = SAI_ROUTER_INTERFACE_TYPE_VLAN;
    attrIdx++; // attribute 2
    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_VLAN_ID; // entry 2
    rif_attr[attrIdx].value.u16 = vlanId;
    attrIdx++; // attribute 3
    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_SRC_MAC_ADDRESS; // entry 3
    memcpy(rif_attr[attrIdx].value.mac, switchMacAddr, MAC_ADDR_LEN);
    attrIdx++; // attribute 4
    rv = sai_rif_api_tbl->create_router_interface(&vlan_rif_oid[vlanId], attrIdx, rif_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create ip router interface on vlan %d", vlanId);
        return -1;
    }
#endif
    return 0;
}

int SaiCreateIPIntfLoopback(uint32_t *ipAddr, int maskLen, int ifId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    int attrIdx = 0;
    sai_attribute_t rif_attr[3];
    sai_object_id_t rid;

    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_VIRTUAL_ROUTER_ID; //entry 0
    rif_attr[attrIdx].value.oid = globalVrId;
    attrIdx++; // attribute 1
    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_TYPE; // entry 1
    rif_attr[attrIdx].value.s32 = SAI_ROUTER_INTERFACE_TYPE_LOOPBACK;
    attrIdx++; // attribute 2
    rif_attr[attrIdx].id = SAI_ROUTER_INTERFACE_ATTR_SRC_MAC_ADDRESS; // entry 2
    memcpy(rif_attr[attrIdx].value.mac, switchMacAddr, MAC_ADDR_LEN);
    attrIdx++; // attribute 3
    rv = sai_rif_api_tbl->create_router_interface(&rid, attrIdx, rif_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create loopback ip router interface (%d)", ifId);
        return -1;
    }
    syslog(LOG_INFO, "Create loopback router interface (%d)", ifId);
    loopback_rif_oid[ifId] = rid;
#endif
    return 0;
}

/* Delete interface route */
static int SaiDeleteInterfaceRoute(const uint32_t ipAddr)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_unicast_route_entry_t uc_route_entry = {0};

    uc_route_entry.vr_id = globalVrId;
    uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
    uc_route_entry.destination.addr.ip4 = htonl(ipAddr);
    uc_route_entry.destination.mask.ip4 = 0xFFFFFFFF;

    syslog(LOG_INFO, "Deleting interface route to cpu for %x", ipAddr);
    rv = sai_route_api_tbl->remove_route(&uc_route_entry);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure deleting interface route to cpu for - %x", ipAddr);
        return -1;
    }
#endif
    return 0;
}

int SaiDeleteIPIntf(uint32_t *ipAddr, int maskLen, int vlanId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    if (vlan_rif_oid[vlanId] != 0) {
        rv = sai_rif_api_tbl->remove_router_interface(vlan_rif_oid[vlanId]);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to delete ipv4 router interface %x/%d on vlan %d", ipAddr[0], maskLen, vlanId);
            return -1;
        }
        vlan_rif_oid[vlanId] = 0;
    }
#endif
    return 0;
}

int SaiDeleteIPIntfLoopback(uint32_t *ipAddr, int maskLen, int ifId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    if (loopback_rif_oid[ifId] != 0) {
        rv = sai_rif_api_tbl->remove_router_interface(loopback_rif_oid[ifId]);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to delete loopback ipv4 router interface %x/%d (%d)", ipAddr[0], maskLen, ifId);
            return -1;
        }
        syslog(LOG_INFO, "Delete loopback router interface (%d)", ifId);
        loopback_rif_oid[ifId] = 0;
    }
#endif
    return 0;
}

uint64_t SaiCreateIPNextHop(uint32_t *ipAddr, uint32_t nextHopFlags,
        int vlanId, int routerPhyIntf, uint8_t *macAddr, int ip_type)
{
#ifdef SAI_BUILD
    int i = 0, j = 0;
    sai_status_t rv;
    sai_attribute_t attr[3];
    sai_object_id_t rifId, nextHopId;

    //Initialize rifId
    rifId = vlan_rif_oid[vlanId];

    //Create next hop entry
    attr[0].id = SAI_NEXT_HOP_ATTR_TYPE;
    attr[0].value.s32 = SAI_NEXT_HOP_IP;
    attr[1].id = SAI_NEXT_HOP_ATTR_IP;

    switch (ip_type) {
        case IP_TYPE_IPV4:
            attr[1].value.ipaddr.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
            attr[1].value.ipaddr.addr.ip4 = htonl(ipAddr[0]);
            break;
        case IP_TYPE_IPV6:
            attr[1].value.ipaddr.addr_family = SAI_IP_ADDR_FAMILY_IPV6;
            for (i = 0; i < 4; i++) {
                uint32_t ip = htonl(ipAddr[i]);
                int k = 0;
                while (k < 4) {
                    attr[1].value.ipaddr.addr.ip6[j] = ip >> (k*8);
                    k++;
                    j++;
                }
            }
            break;
    }

    attr[2].id = SAI_NEXT_HOP_ATTR_ROUTER_INTERFACE_ID;
    attr[2].value.oid = rifId;

    rv = sai_nh_api_tbl->create_next_hop(&nextHopId, 3, attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR,
               "Failed to create next hop entry for ip %x on vlan, port %d, %d",
               *ipAddr, vlanId, routerPhyIntf);
    }

    return ((uint64_t)nextHopId);
#endif
    return 0;
}

int SaiDeleteIPNextHop(uint64_t nextHopId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    //Remove nexthop entry
    rv = sai_nh_api_tbl->remove_next_hop(nextHopId);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove next hop entry");
        return -1;
    }
#endif
    return 0;
}

int SaiUpdateIPNextHop(uint32_t ipAddr, uint64_t nextHopId, int vlanId,
        int routerPhyIntf, uint8_t *macAddr)
{
    return 0;
}

int SaiRestoreIPv4NextHop()
{
    return 0;
}

uint64_t SaiCreateIPNextHopGroup(int numOfNh, uint64_t *nhIdArr)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    uint64_t ecmpGrpId;
    sai_attribute_t attr[2];

    attr[0].id = SAI_NEXT_HOP_GROUP_ATTR_TYPE;
    attr[0].value.s32 = SAI_NEXT_HOP_GROUP_ECMP;
    attr[1].id = SAI_NEXT_HOP_GROUP_ATTR_NEXT_HOP_LIST;
    attr[1].value.objlist.count = numOfNh;
    attr[1].value.objlist.list = nhIdArr;
    rv = sai_nh_grp_api_tbl->create_next_hop_group(&ecmpGrpId, 2, attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create next hop group");
        return INVALID_OBJECT_ID;
    }
    return ecmpGrpId;
#endif
    return 0;
}

int SaiDeleteIPNextHopGroup(uint64_t ecmpGrpId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    rv = sai_nh_grp_api_tbl->remove_next_hop_group(ecmpGrpId);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to destroy ecmp group");
        return -1;
    }
#endif
    return 0;
}

int SaiUpdateIPNextHopGroup(int numOfNh, uint64_t *nhIdArr, uint64_t ecmpGrpId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_attribute_t attr;

    attr.id = SAI_NEXT_HOP_GROUP_ATTR_NEXT_HOP_LIST;
    attr.value.objlist.count = numOfNh;
    attr.value.objlist.list = nhIdArr;
    rv = sai_nh_grp_api_tbl->set_next_hop_group_attribute(ecmpGrpId, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to update next hop group");
        return -1;
    }
#endif
    return 0;
}


static void populateIpAddrInfo(const uint32_t *ipAddr, const int ip_type,
        sai_neighbor_entry_t *neighborEntry)
{
#ifdef SAI_BUILD
    int i = 0, j = 0;

    switch (ip_type) {
        case IP_TYPE_IPV4:
            neighborEntry->ip_address.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
            neighborEntry->ip_address.addr.ip4 = htonl(ipAddr[0]);
            break;
        case IP_TYPE_IPV6:
            neighborEntry->ip_address.addr_family = SAI_IP_ADDR_FAMILY_IPV6;
            for (i = 0; i < 4; i++) {
                uint32_t ip = htonl(ipAddr[i]);
                int k = 0;
                while (k < 4) {
                    neighborEntry->ip_address.addr.ip6[j] = ip >> (k*8);
                    k++;
                    j++;
                }
            }
            break;
    }
#endif
}

int SaiCreateIPNeighbor(uint32_t *ipAddr, uint32_t neighborFlags,
        uint8_t *macAddr, int vlanId, int ip_type)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_object_id_t rifId;
    sai_attribute_t attr[3];
    int attrCount = 0;
    sai_neighbor_entry_t neighborEntry;

    //Initialize rifId
    rifId = vlan_rif_oid[vlanId];

    //Initialize neighbor entry
    neighborEntry.rif_id = rifId;
    populateIpAddrInfo(ipAddr, ip_type, &neighborEntry);
    attr[0].id = SAI_NEIGHBOR_ATTR_DST_MAC_ADDRESS;

    if (neighborFlags & NEIGHBOR_TYPE_BLACKHOLE) {
        //Drop packets destined to null intf
        memcpy(attr[0].value.mac, switchMacAddr, MAC_ADDR_LEN);
        /**
         * bcm-sai 2.1.5.1 not yet support SAI_NEIGHBOR_ATTR_PACKET_ACTION
         */
        //attr[1].id = SAI_NEIGHBOR_ATTR_PACKET_ACTION;
        //attr[1].value.s32 = SAI_PACKET_ACTION_DENY;
        attrCount = 1;
        syslog(LOG_INFO, "Setting neighbor attribute DENY");
    } else if (neighborFlags & NEIGHBOR_TYPE_COPY_TO_CPU) {
        //Trap to CPU packets destined to intf IP
        memcpy(attr[0].value.mac, switchMacAddr, MAC_ADDR_LEN);
        /**
         * bcm-sai 2.1.5.1 not yet support SAI_NEIGHBOR_ATTR_PACKET_ACTION
         */
        //attr[1].id = SAI_NEIGHBOR_ATTR_PACKET_ACTION;
        //attr[1].value.s32 = SAI_PACKET_ACTION_TRAP;
        attrCount = 1;
        syslog(LOG_INFO, "Setting neighbor attribute TRAP");
    } else if (neighborFlags & NEIGHBOR_TYPE_FULL_SPEC_NEXTHOP) {
        memcpy(attr[0].value.mac, macAddr, MAC_ADDR_LEN);
        attrCount = 1;
    }

    rv = sai_nbr_api_tbl->create_neighbor_entry(&neighborEntry, attrCount,
                                                attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create neighbor entry for ip");
        return -1;
    }
#endif
    return 0;
}

int SaiUpdateIPNeighbor(uint32_t *ipAddr, uint8_t *macAddr, int vlanId,
        int ip_type)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_attribute_t attr;
    sai_object_id_t rifId;
    sai_neighbor_entry_t neighborEntry;

    //Initialize rifId
    rifId = vlan_rif_oid[vlanId];

    //Initialize neighbor entry
    neighborEntry.rif_id = rifId;
    populateIpAddrInfo(ipAddr, ip_type, &neighborEntry);

    attr.id = SAI_NEIGHBOR_ATTR_DST_MAC_ADDRESS;
    memcpy(attr.value.mac, macAddr, MAC_ADDR_LEN);
    rv = sai_nbr_api_tbl->set_neighbor_attribute(&neighborEntry, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to update neighbor MAC for neighbor ip");
        return -1;
    }

    /**
     * bcm-sai 2.1.5.1 not yet support SAI_NEIGHBOR_ATTR_PACKET_ACTION
     */
    /*
    attr.id = SAI_NEIGHBOR_ATTR_PACKET_ACTION;
    attr.value.s32 = SAI_PACKET_ACTION_FORWARD;
    rv = sai_nbr_api_tbl->set_neighbor_attribute(&neighborEntry, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR,
               "Failed to update neighbor packet action to forward for neighbor ip");
        return -1;
    }
    */
#endif
    return 0;
}

int SaiDeleteIPNeighbor(uint32_t *ipAddr, uint8_t *macAddr, int vlanId,
        int ip_type)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_object_id_t rifId;
    sai_neighbor_entry_t neighborEntry;

    //Initialize rifId
    rifId = vlan_rif_oid[vlanId];
    //Initialize neighbor entry
    neighborEntry.rif_id = rifId;
    populateIpAddrInfo(ipAddr, ip_type, &neighborEntry);

    //Remove neighbor table entry
    rv = sai_nbr_api_tbl->remove_neighbor_entry(&neighborEntry);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove neighbor entry for neighbor ip %x",
               *ipAddr);
        return -1;
    }
#endif
    return 0;
}

//FIXME: Neighbor table/Next hop table support for warmboot needs to be implemented
int SaiIPv4NeighborTblCB()
{
    return 0;
}

int SaiRestoreIPNeighborDB()
{
    return 0;
}

int SaiDeleteIPRoute(uint8_t *ipPrefix, uint8_t *ipMask, uint32_t routeFlags)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_unicast_route_entry_t uc_route_entry;

    memset(&uc_route_entry, 0, sizeof(sai_unicast_route_entry_t));
    uc_route_entry.vr_id = globalVrId;

    if (routeFlags & ROUTE_TYPE_V6) {
        uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV6;
        memcpy(uc_route_entry.destination.addr.ip6, ipPrefix,
               sizeof(sai_ip6_t));
        memcpy(uc_route_entry.destination.mask.ip6, ipMask, sizeof(sai_ip6_t));
    } else {
        uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
        uc_route_entry.destination.addr.ip4 = htonl((uint32_t)(ipPrefix[3]) |
                                                    (uint32_t)(ipPrefix[2])<<8 |
                                                    (uint32_t)(ipPrefix[1])<<16 |
                                                    (uint32_t)(ipPrefix[0])<<24);
        uc_route_entry.destination.mask.ip4 = htonl((uint32_t)(ipMask[3]) |
                                                    (uint32_t)(ipMask[2])<<8 |
                                                    (uint32_t)(ipMask[1])<<16 |
                                                    (uint32_t)(ipMask[0])<<24);
    }

    rv = sai_route_api_tbl->remove_route(&uc_route_entry);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure deleting IP route");
        return -1;
    }
#endif
    return 0;
}

int SaiCreateIPRoute(uint8_t *ipPrefix, uint8_t *ipMask, uint32_t routeFlags,
        uint64_t nextHopId, int rifId)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_unicast_route_entry_t uc_route_entry;
    sai_attribute_t route_attr;

    memset (&uc_route_entry, 0, sizeof(uc_route_entry));
    uc_route_entry.vr_id = globalVrId;

    if (routeFlags & ROUTE_TYPE_V6) {
        uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV6;
        memcpy(uc_route_entry.destination.addr.ip6, ipPrefix,
               sizeof(sai_ip6_t));
        memcpy(uc_route_entry.destination.mask.ip6, ipMask, sizeof(sai_ip6_t));
    } else {
        uc_route_entry.destination.addr_family = SAI_IP_ADDR_FAMILY_IPV4;
        uc_route_entry.destination.addr.ip4 = htonl((uint32_t)(ipPrefix[3]) |
                                                    (uint32_t)(ipPrefix[2])<<8 |
                                                    (uint32_t)(ipPrefix[1])<<16 |
                                                    (uint32_t)(ipPrefix[0])<<24);
        uc_route_entry.destination.mask.ip4 = htonl((uint32_t)(ipMask[3]) |
                                                    (uint32_t)(ipMask[2])<<8 |
                                                    (uint32_t)(ipMask[1])<<16 |
                                                    (uint32_t)(ipMask[0])<<24);
    }

    if (routeFlags & ROUTE_OPERATION_TYPE_UPDATE) {
        route_attr.id = SAI_ROUTE_ATTR_NEXT_HOP_ID;
        route_attr.value.oid = nextHopId;
        rv = sai_route_api_tbl->set_route_attribute(&uc_route_entry, &route_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to update ecmp group id for ip route");
            return -1;
        }
    } else {
        if (routeFlags & ROUTE_TYPE_NULL) {
            route_attr.id = SAI_ROUTE_ATTR_PACKET_ACTION;
            route_attr.value.s32 = SAI_PACKET_ACTION_DROP;
        } else if (routeFlags & ROUTE_TYPE_CONNECTED) {
            /*FIXME: Enhance for devices that support port/lag rif in addition to vlan rif*/
            route_attr.id = SAI_ROUTE_ATTR_NEXT_HOP_ID;
            route_attr.value.oid = vlan_rif_oid[INTF_ID_FROM_IFINDEX(rifId)];
        } else {
            route_attr.id = SAI_ROUTE_ATTR_NEXT_HOP_ID;
            route_attr.value.oid = nextHopId;
        }
        rv = sai_route_api_tbl->create_route(&uc_route_entry, 1, &route_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to add ip route. Error : %llu",
                   (long long unsigned int)rv);
            return -1;
        }
    }
#endif
    return 0;
}

//FIXME: Route table restoration for warmboot needs to be implemented
int SaiIPv4RouteTblCB()
{
    return 0;
}

int SaiRestoreIPv4RouteDB()
{
    return 0;
}
int SaiRestoreIPv6RouteDB()
{
    return 0;
}

int SaiUpdateSubIPv4Intf(uint32_t ipAddr, bool state)
{
    if (state) {
        return (SaiAddInterfaceRoute(ipAddr));
    } else {
        return (SaiDeleteInterfaceRoute(ipAddr));
    }
}

void SaiL3Init()
{
    //Init loopback_rif_oid[]
    memset(&loopback_rif_oid, 0, sizeof(sai_object_id_t)*MAX_LOOPBACK_INTFS);
}
