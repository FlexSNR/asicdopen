#include "saiAcl.h"

#define TRAP_GROUP_SIZE 5

/**
 * trap group map:queue:protocol
 * 0:0: ttl error
 * 1:4: bgp, lacp
 * 2:4: arp req, arp resp, neigh discovery
 * 3:4: lldp dhcp
 * 4:1: ip2me
 */

typedef struct SaiCoppTrapGruop {
    int trap_group_index;
    sai_hostif_trap_id_t trap_id;
} SaiCoppTrapGruop_t;

SaiCoppTrapGruop_t coppTrapMap[] = {
    {0, SAI_HOSTIF_TRAP_ID_TTL_ERROR},
//  {1, SAI_HOSTIF_TRAP_ID_BGP},
    {1, SAI_HOSTIF_TRAP_ID_LACP},
    {2, SAI_HOSTIF_TRAP_ID_ARP_REQUEST},
    {2, SAI_HOSTIF_TRAP_ID_ARP_RESPONSE},
//  {2, SAI_HOSTIF_TRAP_ID_IPV6_NEIGHBOR_DISCOVERY},
    {3, SAI_HOSTIF_TRAP_ID_VRRP},
    {3, SAI_HOSTIF_TRAP_ID_LLDP},
    {3, SAI_HOSTIF_TRAP_ID_STP},
    {3, SAI_HOSTIF_TRAP_ID_PVRST},
    {3, SAI_HOSTIF_TRAP_ID_DHCP}
//  {4, SAI_HOSTIF_TRAP_ID_IP2ME},
//
};

uint32_t coppTrapGroupQueueMap[TRAP_GROUP_SIZE] = {0, 4, 4, 4, 1};
sai_object_id_t coppTrapGroupIdList[TRAP_GROUP_SIZE];

sai_policer_api_t *sai_policer_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl;
extern sai_switch_api_t* sai_switch_api_tbl;
extern sai_hostif_api_t* sai_hostif_api_tbl;

/* Confgure COPP for each protocol */
int SaiCoPPConfig()
{
#ifdef SAI_BUILD
    int rv = 0;
    rv = SaiTrapGroupInit();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to init trap group");
        return -1;
    }

    rv = SaiConfigPolicerTrapGroup0();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to set policer to trap group 0");
        return -1;
    }

    rv = SaiConfigPolicerTrapGroup2();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to set policer to trap group 2");
        return -1;
    }

    rv = SaiConfigPolicerTrapGroup4();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to set policer to trap group 4");
        return -1;
    }

    rv = SaiConfigTrapId();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to config trap id to trap group.");
        return -1;
    }
#endif
    return 0;
}

/* Init trap id to trap group */
int SaiTrapGroupInit()
{
#ifdef SAI_BUILD
    int i = 0;
    sai_object_id_t new_trap_id;
    sai_status_t rv;
    sai_attribute_t attr;

    // First trap group is default trap group
    attr.id = SAI_SWITCH_ATTR_DEFAULT_TRAP_GROUP;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to get default trap group, rc=%d", rv);
        return -1;
    }
    coppTrapGroupIdList[0] = attr.value.oid;

    attr.id = SAI_HOSTIF_TRAP_GROUP_ATTR_QUEUE;
    attr.value.u32 = coppTrapGroupQueueMap[0];
    rv = sai_hostif_api_tbl->set_trap_group_attribute(
            coppTrapGroupIdList[0], &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to set default trap group rc=%d", rv);
        return -1;
    }

    // Create new trap group and bind to CPU queue
    for (i = 1; i < TRAP_GROUP_SIZE; i++) {
        attr.id = SAI_HOSTIF_TRAP_GROUP_ATTR_QUEUE;
        attr.value.u32 = coppTrapGroupQueueMap[i];
        rv = sai_hostif_api_tbl->create_hostif_trap_group(
                &new_trap_id, 1, &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to create trap group rc=%d", rv);
            return -1;
        }
        coppTrapGroupIdList[i] = new_trap_id;
    }
#endif
    return 0;
}

/* Config traps id to trap group */
int SaiConfigTrapId()
{
    int i = 0;
    int trap_len = sizeof(coppTrapMap)/sizeof(*coppTrapMap);
    sai_attribute_t attr;
    sai_status_t rv;

    syslog(LOG_ERR, "Trap len %d", trap_len);

    for (i = 0; i < trap_len; i++) {
        /* Setting PACKET ACTION Trap */
        attr.id = SAI_HOSTIF_TRAP_ATTR_PACKET_ACTION;
        attr.value.s32 = SAI_PACKET_ACTION_TRAP;
        rv = sai_hostif_api_tbl->set_trap_attribute(coppTrapMap[i].trap_id,
                &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set packet action trap attribute %d, "
                    "rv : 0x%x", coppTrapMap[i].trap_id, rv);
            return -1;
        }

        /* Setting TRAP GROUP */
        attr.id = SAI_HOSTIF_TRAP_ATTR_TRAP_GROUP;
        attr.value.oid =
            coppTrapGroupIdList[coppTrapMap[i].trap_group_index];
        rv = sai_hostif_api_tbl->set_trap_attribute(coppTrapMap[i].trap_id,
                &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set trap group attribute %d, "
                    "rv : 0x%x", coppTrapMap[i].trap_id, rv);
            return -1;
        }

        /* Setting TRAP CHANNEL */
        attr.id = SAI_HOSTIF_TRAP_ATTR_TRAP_CHANNEL;
        attr.value.s32 = SAI_HOSTIF_TRAP_CHANNEL_NETDEV;
        rv = sai_hostif_api_tbl->set_trap_attribute(coppTrapMap[i].trap_id,
                &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR,"Failed to set channel trap attribute %d, "
                    "rv : 0x%x\n", coppTrapMap[i].trap_id, rv);
            return -1;
        }

    }
}

int SaiConfigPolicerTrapGroup0()
{
#ifdef SAI_BUILD
    int attrIdx = 0;
    sai_status_t rv;
    sai_attribute_t policer_attr[5];
    sai_attribute_t attr;
    sai_object_id_t policer_id;

    policer_attr[attrIdx].id = SAI_POLICER_ATTR_METER_TYPE;
    policer_attr[attrIdx].value.s32 = SAI_METER_TYPE_PACKETS;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_MODE;
    policer_attr[attrIdx].value.s32 = SAI_POLICER_MODE_Sr_TCM;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_CBS;
    policer_attr[attrIdx].value.u64 = 600;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_CIR;
    policer_attr[attrIdx].value.u64 = 600;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_RED_PACKET_ACTION;
    policer_attr[attrIdx].value.s32 = SAI_PACKET_ACTION_DROP;
    attrIdx++;

    rv = sai_policer_api_tbl->create_policer(
            &policer_id, attrIdx, policer_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "COPP: Failed to create policer for Trap Group 0");
        return -1;
    }

    attr.id = SAI_HOSTIF_TRAP_GROUP_ATTR_POLICER;
    attr.value.oid = policer_id;

    rv = sai_hostif_api_tbl->set_trap_group_attribute(coppTrapGroupIdList[0],
            &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to bind policer to trap group 0, rc=%d", rv);
        return -1;
    }
#endif
    return 0;
}

int SaiConfigPolicerTrapGroup2()
{
#ifdef SAI_BUILD
    int attrIdx = 0;
    sai_status_t rv;
    sai_attribute_t policer_attr[5];
    sai_attribute_t attr;
    sai_object_id_t policer_id;

    policer_attr[attrIdx].id = SAI_POLICER_ATTR_METER_TYPE;
    policer_attr[attrIdx].value.s32 = SAI_METER_TYPE_PACKETS;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_MODE;
    policer_attr[attrIdx].value.s32 = SAI_POLICER_MODE_Sr_TCM;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_CBS;
    policer_attr[attrIdx].value.u64 = 600;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_CIR;
    policer_attr[attrIdx].value.u64 = 600;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_RED_PACKET_ACTION;
    policer_attr[attrIdx].value.s32 = SAI_PACKET_ACTION_DROP;
    attrIdx++;

    rv = sai_policer_api_tbl->create_policer(
            &policer_id, attrIdx, policer_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "COPP: Failed to create policer for Trap Group 2");
        return -1;
    }

    attr.id = SAI_HOSTIF_TRAP_GROUP_ATTR_POLICER;
    attr.value.oid = policer_id;

    rv = sai_hostif_api_tbl->set_trap_group_attribute(coppTrapGroupIdList[2],
            &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to bind policer to trap group 2, rc=%d", rv);
        return -1;
    }
#endif
    return 0;
}

int SaiConfigPolicerTrapGroup4()
{
#ifdef SAI_BUILD
    int attrIdx = 0;
    sai_status_t rv;
    sai_attribute_t policer_attr[5];
    sai_attribute_t attr;
    sai_object_id_t policer_id;

    policer_attr[attrIdx].id = SAI_POLICER_ATTR_METER_TYPE;
    policer_attr[attrIdx].value.s32 = SAI_METER_TYPE_PACKETS;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_MODE;
    policer_attr[attrIdx].value.s32 = SAI_POLICER_MODE_Sr_TCM;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_CBS;
    policer_attr[attrIdx].value.u64 = 600;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_CIR;
    policer_attr[attrIdx].value.u64 = 600;
    attrIdx++;
    policer_attr[attrIdx].id = SAI_POLICER_ATTR_RED_PACKET_ACTION;
    policer_attr[attrIdx].value.s32 = SAI_PACKET_ACTION_DROP;
    attrIdx++;

    rv = sai_policer_api_tbl->create_policer(
            &policer_id, attrIdx, policer_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "COPP: Failed to create policer for Trap Group 4");
        return -1;
    }

    attr.id = SAI_HOSTIF_TRAP_GROUP_ATTR_POLICER;
    attr.value.oid = policer_id;

    rv = sai_hostif_api_tbl->set_trap_group_attribute(coppTrapGroupIdList[4],
            &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to bind policer to trap group 4, rc=%d", rv);
        return -1;
    }
#endif
    return 0;
}

/** COPP stats API */
