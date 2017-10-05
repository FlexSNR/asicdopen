#include <stdio.h>
#include <string.h>
#include <syslog.h>

#include <sai.h>
#include <saitypes.h>

#include "_cgo_export.h"
#include "../pluginCommon/pluginCommon.h"
#include "saiCommon.h"
#include "ingrasys/s9100/sr_ingrasys.h"

sai_port_api_t* sai_port_api_tbl = NULL;
extern sai_hostif_api_t* sai_hostif_api_tbl;

typedef struct portInfo_s {
    char portName[NETIF_NAME_LEN_MAX];
} portInfo_t;

#ifdef SAI_BUILD
portInfo_t portInfo[MAX_NUM_PORTS];
extern knetIfInfo knetIfDB[MAX_NUM_PORTS];

/* Port object id's */
int maxSysPorts = 0;
sai_object_id_t port_list[MAX_NUM_PORTS];
int mediaTypeEnumMap[MAX_MEDIA_TYPE_ENUM];
extern sai_mac_t switchMacAddr;
#endif

/* Translate front panel port number to chip port number */
int SaiXlateFPanelPortToChipPortNum(int port)
{
    return (ingrasys_s9100_portmap[port]);
}

/* Translate front panel port number to chip port number */
int SaiXlateChipPortToFPanelPortNum(int port)
{
    return (ingrasys_s9100_reverse_portmap[port]);
}

/* Translate sai port number to front panel port number */
int SaiXlateSAIPortToFPanelPortNum(int *port_index, sai_object_id_t oid)
{
    int i = 0;
    sai_object_type_t obj_type = SAI_OBJECT_TYPE_NULL;
    obj_type = sai_object_type_query(oid);

    if (obj_type != SAI_OBJECT_TYPE_PORT) {
        syslog(LOG_ERR, "SaiChipPortToLogicPortNum: "
                "Unsupported sai object type");
        return SAI_STATUS_FAILURE;
    }

    for(i = 1; i <= maxSysPorts; i++) {
        if (port_list[i] == oid) {
            *port_index = i;
            return SAI_STATUS_SUCCESS;
        }
    }
    syslog(LOG_ERR, "SaiChipPortToLogicPortNum: "
            "Doesn't find the sai port number in port list.");
    return SAI_STATUS_FAILURE;
}

/* List of call back functions for various events. Needs to be implemented */
void sai_port_state_cb(uint32_t count,
        sai_port_oper_status_notification_t *data)
{
#ifdef SAI_BUILD
    int port, portCount, linkStatus, attrIdx = 0, rdAttrIdx = 0;
    sai_attribute_t port_attr[2];
    sai_attribute_t hostif_attr;
    sai_status_t rv;

    for (portCount = 0; portCount < count; portCount++) {
        port = 0;
        rv = SaiXlateSAIPortToFPanelPortNum(&port, data[portCount].port_id);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to get front panel port number for "
                    "port %d", port);
            continue;
        }

        attrIdx = 0;
        rdAttrIdx = 0;
        port_attr[attrIdx++].id = SAI_PORT_ATTR_SPEED;
        //port_attr[attrIdx++].id = SAI_PORT_ATTR_FULL_DUPLEX_MODE;
        rv = sai_port_api_tbl->get_port_attribute(port_list[port],
                attrIdx, port_attr);

        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to get port speed for port %d", port);
            continue;
        }

        linkStatus = (data[portCount].port_state == SAI_PORT_OPER_STATUS_UP)?
            1 : 0;

        /* FIXME: Add dampening logic to handle flapping links */
        SaiNotifyLinkStateChange(port, linkStatus,
                port_attr[rdAttrIdx++].value.u32, FullDuplex);

        if (knetIfDB[port].valid == 1) {
            hostif_attr.id = SAI_HOSTIF_ATTR_OPER_STATUS;
            hostif_attr.value.booldata = (linkStatus == 1)? true: false;
            rv = sai_hostif_api_tbl->set_hostif_attribute(
                    knetIfDB[port].ifId, &hostif_attr);
            if (rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "Failed to set port %d bcm "
                        "link state %d.", port, linkStatus);
            }
        }
        syslog(LOG_INFO, "Link state change notification sent for "
                "port %d, state %d", port, linkStatus);
    }
#endif
    return;
}

void SaiPortInit(int ifMapCount, ifMapInfo_t *ifMap)
{
#ifdef SAI_BUILD
    int port = 0;
    //Enumerate all front panel ports
    for (port = 1; port <= maxSysPorts; port++) {
        snprintf(portInfo[port].portName, NETIF_NAME_LEN_MAX, "%s%d",
                ifMap[0].ifName, port);
    }
#endif
    return;
}

int SaiGetMaxSysPorts()
{
#ifdef SAI_BUILD
    return (maxSysPorts);
#else
    return 0;
#endif
}

int SaiUpdatePortConfig(int flags, portConfig *info)
{
#ifdef SAI_BUILD
    int port;
    sai_status_t rv;
    sai_attribute_t port_attr;

    port = info->portNum;

    if (flags & PORT_ATTR_AUTONEG) {
        port_attr.id = SAI_PORT_ATTR_AUTO_NEG_MODE;
        port_attr.value.booldata = info->autoneg ? true : false;
        rv = sai_port_api_tbl->set_port_attribute(port_list[port],
                &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set autoneg for port %d", port);
            return -1;
        }
    }

    if (flags & PORT_ATTR_SPEED) {
        /*
           port_attr.id = SAI_PORT_ATTR_ADMIN_STATE;
           port_attr.value.booldata = 0;
           rv = sai_port_api_tbl->set_port_attribute(port_list[port], &port_attr);
           if (rv != SAI_STATUS_SUCCESS) {
           syslog(LOG_ERR, "Failed to set port admin state for port %d", port);
           return -1;
           }
           */
        port_attr.id = SAI_PORT_ATTR_SPEED;
        port_attr.value.u32 = info->portSpeed;
        rv = sai_port_api_tbl->set_port_attribute(port_list[port], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port speed for port %d", port);
            return -1;
        }
        /*
           port_attr.id = SAI_PORT_ATTR_ADMIN_STATE;
           port_attr.value.booldata = 1;
           rv = sai_port_api_tbl->set_port_attribute(port_list[port], &port_attr);
           if (rv != SAI_STATUS_SUCCESS) {
           syslog(LOG_ERR, "Failed to set port admin state for port %d", port);
           return -1;
           }
           */
    }

    /**
     * SAI not yet support set or get port DUPLEX
     */
    /*
       if (flags & PORT_ATTR_DUPLEX) {
       port_attr.id = SAI_PORT_ATTR_FULL_DUPLEX_MODE;
       port_attr.value.booldata = (info->duplex == HalfDuplex) ? false : true;
       rv = sai_port_api_tbl->set_port_attribute(port_list[port], &port_attr);
       if (rv != SAI_STATUS_SUCCESS) {
       syslog(LOG_ERR, "Failed to set duplex mode for port %d", port);
       return -1;
       }
       }
       */
    if (flags & PORT_ATTR_MTU) {
        port_attr.id = SAI_PORT_ATTR_MTU;
        port_attr.value.u32 = info->mtu;
        rv = sai_port_api_tbl->set_port_attribute(port_list[port], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set mtu for port %d", port);
            return -1;
        }
    }

    if (flags & PORT_ATTR_ADMIN_STATE) {
        port_attr.id = SAI_PORT_ATTR_ADMIN_STATE;
        port_attr.value.booldata = info->adminState;
        rv = sai_port_api_tbl->set_port_attribute(port_list[port], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port admin state for port %d, "
                    "rv: 0x%x", port, rv);
            return -1;
        }
    }
#endif
    return 0;
}

int SaiInitPortConfigDB()
{
#ifdef SAI_BUILD
    int port, attrIdx = 0, rdAttrIdx = 0;
    sai_status_t rv;
    sai_attribute_t port_attr[5];
    portConfig info = {0};

    port_attr[attrIdx++].id = SAI_PORT_ATTR_ADMIN_STATE;
    port_attr[attrIdx++].id = SAI_PORT_ATTR_SPEED;
    //port_attr[attrIdx++].id = SAI_PORT_ATTR_FULL_DUPLEX_MODE;
    //sai-2.1.5.1-odp does not support media type
    //port_attr[3].id = SAI_PORT_ATTR_MEDIA_TYPE;
    port_attr[attrIdx++].id = SAI_PORT_ATTR_AUTO_NEG_MODE;
    port_attr[attrIdx++].id = SAI_PORT_ATTR_MTU;

    for (port = 1; port <= maxSysPorts; port++) {
        memset(&info, 0, sizeof(portConfig));
        rdAttrIdx = 0;
        rv = sai_port_api_tbl->get_port_attribute(port_list[port],
                attrIdx, port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to retrieve port attributes for "
                    "port %d, rv=0x%x,  2's complement rv = 0x%x", port, rv, ~rv);
            return -1;
        }

        info.portNum = port;
        /**
         * SAI not yet support set or get port interface type
         * So we default put empty string
         * ex. CR4, KR2, ...
         */
        info.ifType = PortIfTypeCount;
        info.adminState = port_attr[rdAttrIdx++].value.booldata == false ? 0 : 1;
        memcpy(info.macAddr, switchMacAddr, MAC_ADDR_LEN);
        info.portSpeed = port_attr[rdAttrIdx++].value.u32;
        /**
         * SAI not yet support set or get port DUPLEX,
         * So we default put FullDuplex
         */
        //info.duplex = port_attr[rdAttrIdx++].value.booldata == false ? HalfDuplex : FullDuplex;
        info.duplex = FullDuplex;
        /**
         * SAI not yet support get port mediaType,
         * So we default put empty string
         * ex. fiber, copper, (QSFP_fiber, ... ), ...
         */
        //info.media = mediaTypeEnumMap[port_attr[3].value.s32];
        info.media = MediaTypeCount;
        info.autoneg = port_attr[rdAttrIdx++].value.booldata;
        info.mtu = port_attr[rdAttrIdx++].value.u32;
        SaiIntfInitPortConfigDB(&info);
    }
#endif
    return 0;
}

int SaiInitPortStateDB()
{
#ifdef SAI_BUILD
    int port = 1;
    for (port = 1; port <= maxSysPorts; port++) {
        SaiIntfInitPortStateDB(port, portInfo[port].portName);
    }
#endif
    return 0;
}

int SaiUpdatePortStateDB(int startPort, int endPort)
{
#ifdef SAI_BUILD
    int port = 0;
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
        info.portNum = port;
        port_attr.id = SAI_PORT_ATTR_OPER_STATUS;
        rv = sai_port_api_tbl->get_port_attribute(
                port_list[port], 1, &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to retrieve port operstate for "
                    "port %d during port stat db update, rv 0x%x",
                    port, rv);
            return -1;
        }
        info.operState =
            (port_attr.value.s32 == SAI_PORT_OPER_STATUS_UP) ? 1 :0;
        memcpy(info.portName, portInfo[port].portName, NETIF_NAME_LEN_MAX);
        rv = sai_port_api_tbl->get_port_stats(
                port_list[port], statTypeList,
                sizeof(statTypeList)/sizeof(sai_port_stat_counter_t),
                (uint64_t*)info.stats);
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
    syslog(LOG_ERR, "SaiClearPortStat not yet support.");
    return 0;
}
