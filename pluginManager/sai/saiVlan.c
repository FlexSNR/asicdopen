#include <stdio.h>
#include <syslog.h>

#include <sai.h>
#include <saistatus.h>
#include <saitypes.h>

#include "../pluginCommon/pluginCommon.h"

sai_vlan_api_t* sai_vlan_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl;

#ifdef SAI_BUILD
extern sai_object_id_t port_list[MAX_NUM_PORTS];
sai_object_id_t vlan_member_oid[MAX_VLAN_ID-1][MAX_NUM_PORTS];
#endif

int SaiCreateVlan(int vlanId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    rv = sai_vlan_api_tbl->create_vlan(vlanId);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to create vlan - %d", vlanId);
        return -1;
    }
#endif
    return 0;
}

int SaiDeleteVlan(int vlanId)
{
#ifdef SAI_BUILD
    sai_status_t rv;

    rv = sai_vlan_api_tbl->remove_vlan(vlanId);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to remove vlan - %d", vlanId);
        return -1;
    }
#endif
    return 0;
}

int SaiUpdateVlanAddPorts(int vlanId, int portCount, int untagPortCount,
        int *portList, int *untagPortList)
{
#ifdef SAI_BUILD
    int i = 0, port_index = 0;
    sai_status_t rv;
    sai_attribute_t vlan_member_attr[3];
    sai_attribute_t port_attr;

    vlan_member_attr[0].id = SAI_VLAN_MEMBER_ATTR_VLAN_ID;
    vlan_member_attr[0].value.u16 = vlanId;

    for (i = 0; i < portCount; i++) {
        port_index = portList[i];
        vlan_member_attr[1].id = SAI_VLAN_MEMBER_ATTR_PORT_ID;
        vlan_member_attr[1].value.oid = port_list[port_index];
        vlan_member_attr[2].id = SAI_VLAN_MEMBER_ATTR_TAGGING_MODE;
        vlan_member_attr[2].value.s32 = SAI_VLAN_PORT_TAGGED;

        rv = sai_vlan_api_tbl->create_vlan_member(
                &vlan_member_oid[vlanId][port_index], 3, vlan_member_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to add port %d to vlan %d",
                    portList[i], vlanId);
            return -1;
        }
    }

    for (i = 0; i < untagPortCount; i++) {
        port_index = untagPortList[i];
        vlan_member_attr[1].id = SAI_VLAN_MEMBER_ATTR_PORT_ID;
        vlan_member_attr[1].value.oid = port_list[port_index];
        vlan_member_attr[2].id = SAI_VLAN_MEMBER_ATTR_TAGGING_MODE;
        vlan_member_attr[2].value.s32 = SAI_VLAN_PORT_UNTAGGED;
        rv = sai_vlan_api_tbl->create_vlan_member(
                &vlan_member_oid[vlanId][port_index], 3, vlan_member_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to add port %d to vlan %d",
                    untagPortList[i], vlanId);
            return -1;
        }

        port_attr.id = SAI_PORT_ATTR_PORT_VLAN_ID;
        port_attr.value.u16 = vlanId;
        rv = sai_port_api_tbl->set_port_attribute(
                port_list[port_index], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to set pvid of port %d to %d",
                    untagPortList[i], vlanId);
            return -1;
        }
    }
#endif
    return 0;
}

int SaiUpdateVlanDeletePorts(int vlanId, int portCount, int untagPortCount,
        int *portList, int *untagPortList)
{
#ifdef SAI_BUILD
    int i = 0, port_index = 0;
    sai_status_t rv;
    sai_attribute_t port_attr;

    for (i = 0; i < portCount; i++) {
        port_index = portList[i];
        rv = sai_vlan_api_tbl->remove_vlan_member(
                vlan_member_oid[vlanId][port_index]);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to remove port %d from vlan %d",
                    portList[i], vlanId);
            return -1;
        }
        //Clear object id
        vlan_member_oid[vlanId][port_index] = 0;
    }

    for (i = 0; i < untagPortCount; i++) {
        port_index = untagPortList[i];
        rv = sai_vlan_api_tbl->remove_vlan_member(
                vlan_member_oid[vlanId][port_index]);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to remove port %d from vlan %d",
                    portList[i], vlanId);
            return -1;
        }
        //Clear object id
        vlan_member_oid[vlanId][port_index] = 0;
        //Set port vid to default vlan id
        port_attr.id = SAI_PORT_ATTR_PORT_VLAN_ID;
        port_attr.value.u16 = DEFAULT_VLAN_ID;
        rv = sai_port_api_tbl->set_port_attribute(
                port_list[port_index], &port_attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Unable to restore default pvid for port %d",
                    untagPortList[i]);
            return -1;
        }
    }
#endif
    return 0;
}
