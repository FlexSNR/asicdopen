#include <stdio.h>
#include <string.h>
#include <syslog.h>
#include <sai.h>
#include <saitypes.h>
#include <saistatus.h>
#include <sys/ioctl.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <net/if_arp.h>
#include <net/if.h>
#include "../pluginCommon/pluginCommon.h"

typedef struct knetIfInfo_s {
    int valid;
    int port;
    sai_object_id_t ifId;
    char ifName[NETIF_NAME_LEN_MAX];
} knetIfInfo;
knetIfInfo knetIfDB[MAX_NUM_PORTS];

sai_hostif_api_t* sai_hostif_api_tbl = NULL;
extern int maxSysPorts;
extern sai_object_id_t port_list[MAX_NUM_PORTS];
extern sai_mac_t switchMacAddr;

int SaiCreateLinuxIf(int port, char *ifName, sai_object_id_t *hostIfId) {
#ifdef SAI_BUILD
    char cmd[64];
    int cport;
    sai_status_t rv;
    struct ifreq ifr = {0};
    int sock, errno;
    sai_attribute_t sai_hostif_attr[3];

    cport = SaiXlateFPanelPortToChipPortNum(port);
    sai_hostif_attr[0].id = SAI_HOSTIF_ATTR_TYPE;
    sai_hostif_attr[0].value.s32 = SAI_HOSTIF_TYPE_NETDEV;
    sai_hostif_attr[1].id = SAI_HOSTIF_ATTR_RIF_OR_PORT_ID;
    sai_hostif_attr[1].value.oid = port_list[cport];
    sai_hostif_attr[2].id = SAI_HOSTIF_ATTR_NAME;
    memcpy(sai_hostif_attr[2].value.chardata, ifName, NETIF_NAME_LEN_MAX);
    rv = sai_hostif_api_tbl->create_hostif(hostIfId, 3, sai_hostif_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure mapping asic ports to linux interfaces: port, ifName - %d, %s", port, ifName);
    }

    memcpy(&ifr.ifr_hwaddr.sa_data, switchMacAddr, sizeof(ifr.ifr_hwaddr.sa_data));
    sock = socket(AF_INET, SOCK_DGRAM, 0);
    strncpy(ifr.ifr_name, ifName, strlen(ifName));
    ifr.ifr_hwaddr.sa_family = ARPHRD_ETHER;
    errno = ioctl(sock, SIOCSIFHWADDR, &ifr);
    if (errno < 0) {
        syslog(LOG_ERR,"\t: fail to set ip mac Addr for interface %s. errno=%d %s\n", 
                ifr.ifr_ifrn.ifrn_name, errno, strerror(errno));
    }
#endif
    return 0;
}

int SaiCreateLinuxIfForAsicPorts(int ifMapCount, ifMapInfo_t *ifMap)
{
#ifdef SAI_BUILD
    int i;
    sai_status_t rv;
    sai_object_id_t ifId;
    knetIfInfo knetIfDBEntry;
    char ifName[NETIF_NAME_LEN_MAX];

    if (ifMapCount == -1) {
        // Map all front panel ports to linux interfaces
        for (i = 1; i <= maxSysPorts; i++) {
            snprintf(ifName, NETIF_NAME_LEN_MAX, "%s%d", ifMap[0].ifName, i);
            rv = SaiCreateLinuxIf(i, ifName, &ifId);
            if (rv < 0) {
                return -1;
            }
            //Save knet if info to database
            knetIfDBEntry.valid = 1;
            knetIfDBEntry.ifId = ifId;
            strncpy(knetIfDBEntry.ifName, ifName, NETIF_NAME_LEN_MAX);
            knetIfDBEntry.ifName[NETIF_NAME_LEN_MAX - 1] = '\0';
            knetIfDB[i] = knetIfDBEntry;
        }
    } else {
        for (i = 0; i < ifMapCount; i++) {
            rv = SaiCreateLinuxIf(ifMap[i].port, ifMap[i].ifName, &ifId);
            if (rv < 0) {
                return -1;
            }
            //Save knet if info to database
            knetIfDBEntry.valid = 1;
            knetIfDBEntry.ifId = ifId;
            strncpy(knetIfDBEntry.ifName, ifName, NETIF_NAME_LEN_MAX);
            knetIfDBEntry.ifName[NETIF_NAME_LEN_MAX - 1] = '\0';
            knetIfDB[ifMap[i].port] = knetIfDBEntry;
        }
    }
#endif
    return 0;
}

int SaiDeleteLinuxIfForAsicPorts(int ifMapCount, ifMapInfo_t *ifMap)
{
#ifdef SAI_BUILD
    int i, dbIndex;
    sai_status_t rv;
    knetIfInfo knetIfDBEntry;
    int localIfMapCount = (ifMapCount == ALL_INTERFACES) ? MAX_NUM_PORTS: ifMapCount;

    memset(&knetIfDBEntry, 0, sizeof(knetIfDBEntry));
    for (i = 0; i < localIfMapCount; i++) {
        if (ifMapCount == ALL_INTERFACES) {
            dbIndex = i;
            //Skip invalid entries
            if (!knetIfDB[dbIndex].valid) {
                continue;
            }
        } else {
            dbIndex = ifMap[i].port;
        }
        if ((ifMapCount == ALL_INTERFACES) || !strncmp(knetIfDB[dbIndex].ifName, ifMap[i].ifName, NETIF_NAME_LEN_MAX)) {
            rv = sai_hostif_api_tbl->remove_hostif(knetIfDB[dbIndex].ifId);
            if (rv != SAI_STATUS_SUCCESS) {
                syslog(LOG_ERR, "Failed to delete net interface mapping for port, ifName - %d, %s.", dbIndex, knetIfDB[dbIndex].ifName);
            }
            //Delete knet if info from database
            knetIfDB[dbIndex] = knetIfDBEntry;
        } else {
            syslog(LOG_ERR, "Invalid interface map arguments when attempting to delete net interface : port, ifName - %d, %s", ifMap[i].port, ifMap[i].ifName);
        }
    }
#endif
    return 0;
}
