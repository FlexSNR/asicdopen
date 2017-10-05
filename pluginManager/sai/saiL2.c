#include <stdio.h>
#include <string.h>
#include <syslog.h>

#include <sai.h>
#include <saifdb.h>
#include <saistatus.h>
#include <saistp.h>
#include <saitypes.h>

#include "_cgo_export.h"
#include "../pluginCommon/pluginCommon.h"

sai_fdb_api_t* sai_fdb_api_tbl = NULL;
sai_stp_api_t* sai_stp_api_tbl = NULL;
#ifdef SAI_BUILD

extern int maxSysPorts;
extern sai_object_id_t port_list[MAX_NUM_PORTS];
/* Record original stp id in SAI */
sai_object_id_t stg_id_list[MAX_VLAN_ID];
#endif

void sai_fdb_cb(uint32_t count, sai_fdb_event_notification_data_t *data)
{
#ifdef SAI_BUILD
    int idx, op, attrIdx, port, portIdx;
    sai_status_t rv;

    for (idx = 0; idx < count; idx++) {
        if (data[idx].event_type == SAI_FDB_EVENT_LEARNED) {
            op = MAC_ENTRY_LEARNED;
        } else if (data[idx].event_type == SAI_FDB_EVENT_AGED) {
            op = MAC_ENTRY_AGED;
        } else {
            syslog(LOG_ERR, "Unsupported FDB call back operation");
            continue;
        }

        rv = SAI_STATUS_FAILURE;
        for (attrIdx = 0; attrIdx < data[idx].attr_count; attrIdx++) {
            if (data[idx].attr[attrIdx].id == SAI_FDB_ENTRY_ATTR_PORT_ID) {
                rv = SaiXlateSAIPortToFPanelPortNum(
                        &port, data[idx].attr[attrIdx].value.oid);
                break;
            }
        }
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unsupported sai object type");
        }

        SaiNotifyFDBEvent(op, port, data[idx].fdb_entry.vlan_id,
                data[idx].fdb_entry.mac_address);
        syslog(LOG_INFO, "FDB event sent for port %d, vlan %d, op %d",
                port, data[idx].fdb_entry.vlan_id, op);
    }
#endif
}

int SaiAddRsvdProtoMacEntry(uint8_t *macAddr, uint8_t *mask, int vlanid)
{
    return 0;
}

int SaiDeleteRsvdProtoMacEntry(uint8_t *macAddr, uint8_t *mask, int vlanid)
{
    return 0;
}

int SaiCreateStg(int listLen, int *vlanList)
{
#ifdef SAI_BUILD
    int i = 0, attr_count = 0;
    sai_status_t rv;
    sai_object_id_t stg_id;
    sai_attribute_t stp_attr;

    stp_attr.value.vlanlist.list = NULL;
    if (listLen != 0) {
        stp_attr.id = SAI_STP_ATTR_VLAN_LIST;
        stp_attr.value.vlanlist.count = listLen;
        stp_attr.value.vlanlist.list = malloc(sizeof(sai_vlan_id_t) * listLen);
        if (stp_attr.value.vlanlist.list == NULL) {
            syslog(LOG_CRIT, "Failed to allocate memory in SaiCreateStg.");
            return -1;
        }
        for (i = 0; i < listLen; i++) {
            stp_attr.value.vlanlist.list[i] = vlanList[i];
        }
        attr_count++;
    }
    rv = sai_stp_api_tbl->create_stp(&stg_id, attr_count, &stp_attr);
    if (stp_attr.value.vlanlist.list != NULL) {
        free(stp_attr.value.vlanlist.list);
    }
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create STG.");
        return -1;
    }
    stg_id_list[(int)stg_id] = stg_id;
    return (int)stg_id;
#endif
    return 0;
}

int SaiDeleteStg(int stgId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    rv = sai_stp_api_tbl->remove_stp(stg_id_list[stgId]);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove STG.");
        return -1;
    }
    stg_id_list[stgId] = 0;
#endif
    return 0;
}

int SaiSetPortStpState(int stgId, int port, int stpState)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_port_stp_port_state_t port_state;

    switch (stpState) {
        case StpPortStateLearning:
            port_state = SAI_PORT_STP_STATE_LEARNING;
            break;
        case StpPortStateForwarding:
            port_state = SAI_PORT_STP_STATE_FORWARDING;
            break;
        case StpPortStateBlocking:
            port_state = SAI_PORT_STP_STATE_BLOCKING;
            break;
        default:
            syslog(LOG_ERR, "No stg state %d can be set.", stpState);
            return -1;
    }

    rv = sai_stp_api_tbl->set_stp_port_state(stg_id_list[stgId],
            port_list[port], port_state);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to set port %d to stg state %d.",
                port, stpState);
        return -1;
    }
#endif
    return 0;
}

int SaiGetPortStpState(int stgId, int port)
{
#ifdef SAI_BUILD
    sai_status_t rv;
    sai_port_stp_port_state_t port_state;

    rv = sai_stp_api_tbl->get_stp_port_state(stg_id_list[stgId],
                    port_list[port], &port_state);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to get port %d stg state.", port);
        return -1;
    }

    switch (port_state) {
        case SAI_PORT_STP_STATE_LEARNING:
            return StpPortStateLearning;
        case SAI_PORT_STP_STATE_FORWARDING:
            return StpPortStateForwarding;
        case SAI_PORT_STP_STATE_BLOCKING:
            return StpPortStateBlocking;
        default:
            syslog(LOG_ERR, "No stg state %d at port %d.",
                    (int)port_state, port);
            return -1;
    }
#endif
    return 0;
}

int SaiUpdateStgVlanList(int stgId, int oldListLen, int *oldVlanList,
        int newListLen, int *newVlanList)
{
#ifdef SAI_BUILD
    /**
     * FIXME: Get default STG ID (def_stg_id) from switch SAI API
     */
    int vlan_count = 0, i = 0, j = 0, flag = 0;
    sai_vlan_id_t vlan_list[MAX_VLAN_ID];
    sai_status_t rv;
    sai_attribute_t stp_attr;
    sai_object_id_t def_stg_id =
        (((sai_object_id_t)SAI_OBJECT_TYPE_STP_INSTANCE) << 32) | 1;

    /**
     * Find update old vlan to default stg id
     */
    memset(vlan_list, 0, sizeof(sai_vlan_id_t) * MAX_VLAN_ID);
    for (i = 0; i < oldListLen; i++) {
        flag = 0;
        for (j = 0; j < newListLen; j++) {
            if (oldVlanList[i] == newVlanList[j]) {
                flag = 1;
                break;
            }
        }
        if (flag == 0) {
            vlan_list[vlan_count] = oldVlanList[i];
            vlan_count++;
        }
    }
    stp_attr.value.vlanlist.list = vlan_list;
    stp_attr.value.vlanlist.count = vlan_count;

    if (vlan_count != 0) {
        rv = sai_stp_api_tbl->set_stp_attribute(def_stg_id, &stp_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set VLAN to default stg");
            return -1;
        }
    }

    /**
     * Update new VLANs into stg
     */
    memset(vlan_list, 0, sizeof(sai_vlan_id_t) * MAX_VLAN_ID);
    for (i = 0; i < newListLen; i++) {
        vlan_list[i] = newVlanList[i];
    }
    stp_attr.value.vlanlist.list = vlan_list;
    stp_attr.value.vlanlist.count = newListLen;

    if (vlan_count != 0) {
        rv = sai_stp_api_tbl->set_stp_attribute(stg_id_list[stgId], &stp_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set VLANs to stg %d", stgId);
            return -1;
        }
    }
#endif
    return 0;
}

int SaiFlushFdbByPortVlan(int vlan, int port)
{
#ifdef SAI_BUILD
    int attr_count = 0;
    sai_status_t rv;
    sai_attribute_t fdb_attr[3];

    syslog(LOG_ERR, "Failed to flush vlan %d port %d.", vlan, port);
    fdb_attr[attr_count].id = SAI_FDB_FLUSH_ATTR_ENTRY_TYPE;
    fdb_attr[attr_count].value.s32 = SAI_FDB_FLUSH_ENTRY_DYNAMIC;
    attr_count++;

    if (vlan != -1) {
        fdb_attr[attr_count].id = SAI_FDB_FLUSH_ATTR_VLAN_ID;
        fdb_attr[attr_count].value.u16 = vlan;
        attr_count++;
    }

    if (port != -1) {
        fdb_attr[attr_count].id = SAI_FDB_FLUSH_ATTR_PORT_ID;
        fdb_attr[attr_count].value.oid = port_list[port];
        attr_count++;
    }
    rv = sai_fdb_api_tbl->flush_fdb_entries(attr_count, fdb_attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to flush fdb.");
        return -1;
    }
#endif
    return 0;
}
