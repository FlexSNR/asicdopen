#include <stdio.h>
#include <string.h>
#include <syslog.h>
#include <sai.h>
#include <saitypes.h>
#include "_cgo_export.h"
#include "../pluginCommon/pluginCommon.h"
#ifdef MLNX_SAI
#include "./mlnx/inc/sr_mlnx.h"
#endif
#ifdef CAVM_SAI
#include "./cavm/inc/sr_cavm.h"
#endif

sai_port_api_t* sai_port_api_tbl = NULL;

typedef struct portInfo_s {
    char portName[NETIF_NAME_LEN_MAX];
} portInfo_t;
portInfo_t portInfo[MAX_NUM_PORTS];
/* Port object id's */
int maxSysPorts = 0;
sai_object_id_t port_list[MAX_NUM_PORTS];
int mediaTypeEnumMap[MAX_MEDIA_TYPE_ENUM];
extern sai_mac_t switchMacAddr;

/* Translate front panel port number to chip port number */
int SaiXlateFPanelPortToChipPortNum(int port) {
#ifdef MLNX_SAI
    //FIXME: Enhance this to determine map to lookup based on platform type
    return (mlnx_sn2700_portmap[port]);
#endif
#ifdef CAVM_SAI
    return (cavm_cnx_880091_portmap[port]);
#endif
    return port;
}

/* Translate front panel port number to chip port number */
int SaiXlateChipPortToFPanelPortNum(int port) {
#ifdef MLNX_SAI
    //FIXME: Enhance this to determine map to lookup based on platform type
    return (mlnx_sn2700_reverse_portmap[port]);
#endif
#ifdef CAVM_SAI
    return (cavm_cnx_880091_reverse_portmap[port]);
#endif
    return port;
}

/* List of call back functions for various events. Needs to be implemented */
void sai_port_state_cb (uint32_t count, sai_port_oper_status_notification_t *data)
{
#ifdef SAI_BUILD
    int port, fpPort, portCount, linkStatus, attrIdx = 0, rdAttrIdx = 0;
    sai_attribute_t port_attr[2];
    sai_status_t rv;

    for (portCount = 0; portCount < count; portCount++) {
        for (port = 0; port <= maxSysPorts; port++) {
            if (port_list[port] == data[portCount].port_id ) {
                attrIdx = 0;
                rdAttrIdx = 0;
                port_attr[attrIdx++].id = SAI_PORT_ATTR_SPEED;
#ifndef CAVM_SAI
                port_attr[attrIdx++].id = SAI_PORT_ATTR_FULL_DUPLEX_MODE;
#endif
                rv = sai_port_api_tbl->get_port_attribute(port_list[port], attrIdx, port_attr);
                if (rv != SAI_STATUS_SUCCESS) {
                    syslog(LOG_ERR, "Failed to get port speed, duplex for port %d", port);
                }
                linkStatus = (data[portCount].port_state == SAI_PORT_OPER_STATUS_UP) ? 1 : 0;
                fpPort = SaiXlateChipPortToFPanelPortNum(port);
                syslog(LOG_INFO, "Link state change notification sent for port %d, state %d", fpPort, linkStatus);
                /* FIXME: Add dampening logic to handle flapping links */
#ifdef CAVM_SAI
                // CAVIUM is not supporting full duplex right now and hence sending HalfDuplex
                SaiNotifyLinkStateChange(fpPort, linkStatus, port_attr[rdAttrIdx++].value.u32, FullDuplex);
                // (port_attr[rdAttrIdx++].value.booldata == false) ? HalfDuplex : FullDuplex);
#else
                SaiNotifyLinkStateChange(fpPort, linkStatus, port_attr[rdAttrIdx++].value.u32,
                        (port_attr[rdAttrIdx++].value.booldata == false) ? HalfDuplex : FullDuplex);

#endif
            }
        }
    }
#endif
    return;
}

void SaiPortInit(int ifMapCount, ifMapInfo_t *ifMap) {
#ifdef SAI_BUILD
    int port;
    //Enumerate all front panel ports
    for (port = 1; port <= maxSysPorts; port++) {
        snprintf(portInfo[port].portName, NETIF_NAME_LEN_MAX, "%s%d", ifMap[0].ifName, port);
    }
    //Setup enum maps
    //FIXME: Populate enum info correctly
    if (SAI_PORT_MEDIA_TYPE_SFP_COPPER < MAX_IF_TYPE_ENUM) {
        mediaTypeEnumMap[SAI_PORT_MEDIA_TYPE_NOT_PRESENT] = 0;
        mediaTypeEnumMap[SAI_PORT_MEDIA_TYPE_UNKNONWN] = 0;
        mediaTypeEnumMap[SAI_PORT_MEDIA_TYPE_QSFP_FIBER] = PortIfTypeSR4;
        mediaTypeEnumMap[SAI_PORT_MEDIA_TYPE_QSFP_COPPER] = PortIfTypeCR4;
        mediaTypeEnumMap[SAI_PORT_MEDIA_TYPE_SFP_FIBER] = PortIfTypeSR;
        mediaTypeEnumMap[SAI_PORT_MEDIA_TYPE_SFP_COPPER] = PortIfTypeCR;
    }
#endif
    return;
}

int SaiGetMaxSysPorts()
{
    syslog(LOG_INFO, "Max ports query returning max port = %d", maxSysPorts);
    return (maxSysPorts);
}

int SaiUpdatePortConfig(int flags, portConfig *info)
{
#ifdef SAI_BUILD
    int port, cport;
    sai_status_t rv;
    sai_attribute_t port_attr;

    port = info->portNum;
    cport = SaiXlateFPanelPortToChipPortNum(port);

    if (flags & PORT_ATTR_AUTONEG) {
#ifndef CAVM_SAI
        port_attr.id = SAI_PORT_ATTR_AUTO_NEG_MODE;
        port_attr.value.booldata = info->autoneg ? true : false;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set autoneg for port %d", port);
            return -1;
        }
#endif
    }
    if (flags & PORT_ATTR_SPEED) {
#ifdef CAVM_SAI
        port_attr.id = SAI_PORT_ATTR_ADMIN_STATE;
        port_attr.value.booldata = 0;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port admin state for port %d", port);
            return -1;
        }
#endif
        port_attr.id = SAI_PORT_ATTR_SPEED;
        port_attr.value.u32 = info->portSpeed;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port speed for port %d", port);
            return -1;
        }
#ifdef CAVM_SAI
        port_attr.id = SAI_PORT_ATTR_ADMIN_STATE;
        port_attr.value.booldata = 1;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port admin state for port %d", port);
            return -1;
        }
#endif
    }
    if (flags & PORT_ATTR_DUPLEX) {
#ifndef CAVM_SAI
        //Sai port attribute currently not supported on mlnx
        port_attr.id = SAI_PORT_ATTR_FULL_DUPLEX_MODE;
        port_attr.value.booldata = (info->duplex == HalfDuplex) ? false : true;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set duplex mode for port %d", port);
            return -1;
        }
#endif
    }
    if (flags & PORT_ATTR_MTU) {
        port_attr.id = SAI_PORT_ATTR_MTU;
        port_attr.value.u32 = info->mtu;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set mtu for port %d", port);
            return -1;
        }
    }
    if (flags & PORT_ATTR_ADMIN_STATE) {
        port_attr.id = SAI_PORT_ATTR_ADMIN_STATE;
        port_attr.value.booldata = info->adminState;
        rv = sai_port_api_tbl->set_port_attribute(port_list[cport], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port admin state for port %d, rv: 0x%x", port, rv);
            return -1;
        }
    }
#endif
    return 0;
}

int SaiInitPortConfigDB()
{
#ifdef SAI_BUILD
    int port, cport, attrIdx = 0, rdAttrIdx = 0;
    sai_status_t rv;
    sai_attribute_t port_attr[5];
    portConfig info = {0};

    port_attr[attrIdx++].id = SAI_PORT_ATTR_ADMIN_STATE;
    port_attr[attrIdx++].id = SAI_PORT_ATTR_SPEED;
#ifndef CAVM_SAI
    port_attr[attrIdx++].id = SAI_PORT_ATTR_FULL_DUPLEX_MODE;
#endif
    //MLNX does not support media type
    //port_attr[3].id = SAI_PORT_ATTR_MEDIA_TYPE;
#ifndef CAVM_SAI
    port_attr[attrIdx++].id = SAI_PORT_ATTR_AUTO_NEG_MODE;
#endif
    port_attr[attrIdx++].id = SAI_PORT_ATTR_MTU;
    for (port = 1; port <= maxSysPorts; port++) {
        memset(&info, 0, sizeof(portConfig));
        rdAttrIdx = 0;
        cport = SaiXlateFPanelPortToChipPortNum(port);
        rv = sai_port_api_tbl->get_port_attribute(port_list[cport], attrIdx, port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to retrieve port attributes for port %d, rv=0x%x,  2's complement rv = 0x%x", port, rv, ~rv);
            return -1;
        }
        info.portNum = port;
        info.ifType = PortIfTypeCount;
        info.adminState = port_attr[rdAttrIdx++].value.booldata == false ? 0 : 1;
        memcpy(info.macAddr, switchMacAddr, MAC_ADDR_LEN);
        info.portSpeed = port_attr[rdAttrIdx++].value.u32;
#ifndef CAVM_SAI
        info.duplex = port_attr[rdAttrIdx++].value.booldata == false ? HalfDuplex : FullDuplex;
#endif
        //info.media = mediaTypeEnumMap[port_attr[3].value.s32];
        info.media = MediaTypeCount;
#ifndef CAVM_SAI
        info.autoneg = port_attr[rdAttrIdx++].value.booldata;
#endif
        info.mtu = port_attr[rdAttrIdx++].value.u32;
        SaiIntfInitPortConfigDB(&info);
    }
#endif
    return 0;
}

int SaiInitPortStateDB()
{
#ifdef SAI_BUILD
    int port;
    for (port = 1; port <= maxSysPorts; port++) {
        SaiIntfInitPortStateDB(port, portInfo[port].portName);
    }
#endif
    return 0;
}

int SaiUpdatePortStateDB(int startPort, int endPort)
{
#ifdef SAI_BUILD
    int port, cport;
    portState info;
    sai_status_t rv;
    sai_attribute_t port_attr;
    sai_port_stat_counter_t statTypeList[] = {
        SAI_PORT_STAT_IF_IN_OCTETS,
        SAI_PORT_STAT_IF_IN_UCAST_PKTS,
        SAI_PORT_STAT_IF_IN_DISCARDS,
        SAI_PORT_STAT_IF_IN_ERRORS,
        SAI_PORT_STAT_IF_IN_UNKNOWN_PROTOS,
        SAI_PORT_STAT_IF_OUT_OCTETS,
        SAI_PORT_STAT_IF_OUT_UCAST_PKTS,
        SAI_PORT_STAT_IF_OUT_DISCARDS,
        SAI_PORT_STAT_IF_OUT_ERRORS,
        SAI_PORT_STAT_ETHER_STATS_UNDERSIZE_PKTS,
        SAI_PORT_STAT_ETHER_STATS_OVERSIZE_PKTS,
        SAI_PORT_STAT_ETHER_STATS_FRAGMENTS,
        SAI_PORT_STAT_ETHER_STATS_CRC_ALIGN_ERRORS,
        SAI_PORT_STAT_ETHER_STATS_JABBERS,
        SAI_PORT_STAT_ETHER_STATS_PKTS,
        SAI_PORT_STAT_ETHER_STATS_MULTICAST_PKTS,
        SAI_PORT_STAT_ETHER_STATS_BROADCAST_PKTS,
        SAI_PORT_STAT_ETHER_STATS_PKTS_64_OCTETS,
        SAI_PORT_STAT_ETHER_STATS_PKTS_65_TO_127_OCTETS,
        SAI_PORT_STAT_ETHER_STATS_PKTS_128_TO_255_OCTETS,
        SAI_PORT_STAT_ETHER_STATS_PKTS_256_TO_511_OCTETS,
        SAI_PORT_STAT_ETHER_STATS_PKTS_512_TO_1023_OCTETS,
        SAI_PORT_STAT_ETHER_STATS_PKTS_1024_TO_1518_OCTETS
    };
    if (endPort > maxSysPorts) {
        endPort = maxSysPorts;
    }
    for (port = startPort; port <= endPort; port++) {
        cport = SaiXlateFPanelPortToChipPortNum(port);
        info.portNum = port;
        port_attr.id = SAI_PORT_ATTR_OPER_STATUS;
        rv = sai_port_api_tbl->get_port_attribute(port_list[cport], 1, &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to retrieve port operstate for port %d during port stat db update, rv 0x%x", 
                    port, rv);
            return -1;
        }
        info.operState = (port_attr.value.s32 == SAI_PORT_OPER_STATUS_UP) ? 1 : 0;
        memcpy(info.portName, portInfo[port].portName, NETIF_NAME_LEN_MAX);
        rv = sai_port_api_tbl->get_port_stats(port_list[cport], statTypeList, sizeof(statTypeList)/sizeof(sai_port_stat_counter_t), (uint64_t*)info.stats);
        if (rv < 0) {
            syslog(LOG_ERR, "Failed to retrieve port counters");
            return -1;
        }
        SaiIntfUpdatePortStateDB(&info);
    }
#endif
    return 0;
}

int SaiClearPortStat(int port)
{
#ifdef SAI_BUILD
    int rv = -1;
    
    rv = sai_port_api_tbl->clear_port_all_stats(port);
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to clear the port stats, rv = %d\n", rv); 
    }
#endif
    return 0;
}
