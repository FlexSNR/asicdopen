#include <stdio.h>
#include <string.h>
#include <syslog.h>
#include <stdint.h>
#include <arpa/inet.h>

#include <sai.h>
#include <saitypes.h>
#include <saistatus.h>
#include <saihostintf.h>
#include <saiswitch.h>
#include <sairouter.h>
#include <sairouterintf.h>
#include <saineighbor.h>
#include <sainexthop.h>
#include <sainexthopgroup.h>
#include <sairoute.h>
#include <saiacl.h>
#include <saipolicer.h>

#include "_cgo_export.h"
#include "pluginCommon.h"

/* Definition for API tables */
sai_switch_api_t* sai_switch_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl;
extern sai_hostif_api_t* sai_hostif_api_tbl;
extern sai_vlan_api_t* sai_vlan_api_tbl;
extern sai_fdb_api_t* sai_fdb_api_tbl;
extern sai_virtual_router_api_t* sai_vr_api_tbl;
extern sai_route_api_t* sai_route_api_tbl;
extern sai_router_interface_api_t* sai_rif_api_tbl;
extern sai_neighbor_api_t* sai_nbr_api_tbl;
extern sai_next_hop_api_t* sai_nh_api_tbl;
extern sai_next_hop_group_api_t* sai_nh_grp_api_tbl;
extern sai_buffer_api_t* sai_buffer_api_tbl;
extern sai_acl_api_t* sai_acl_api_tbl;
extern sai_policer_api_t* sai_policer_api_tbl;

sai_mac_t switchMacAddr;
extern sai_object_id_t port_list[MAX_NUM_PORTS];
extern sai_object_id_t globalVrId;
extern int maxSysPorts;

//Forward declarations
int SaiCreateLinuxIfForAsicPorts(int ifMapCount, ifMapInfo_t *ifMap);
int SaiCreateLinuxIf(int port, char *ifName, sai_object_id_t *hostIfId);
int SaiDeleteLinuxIfForAsicPorts(int ifMapCount, ifMapInfo_t *ifMap);

/* List of call back functions for various events. Needs to be implemented */
extern void sai_port_state_cb (uint32_t count, sai_port_oper_status_notification_t *data);

static void sai_port_cb (uint32_t count,
        sai_port_event_notification_t *data)
{
}

extern void sai_fdb_cb (uint32_t count, sai_fdb_event_notification_data_t *data);

static void sai_switch_state_cb (sai_switch_oper_status_t switchstate)
{
}

static void sai_packet_cb (const void *buffer,
        sai_size_t buffer_size,
        uint32_t attr_count,
        const sai_attribute_t *attr_list)
{

}
static void sai_switch_shutdown_cb (void)
{
}


/* Adapter host provided services, currently un-implemented */
const char* profile_get_value(sai_switch_profile_id_t profile_id,
        const char* variable)
{
#if defined(MLNX_SAI)
    /*
     * FIXME: Need to implement a generic mechanism to read in a custom config file
     * and create a config cache. Cache is a map of string key's to string value's
     */
    if (!strncmp(SAI_KEY_INIT_CONFIG_FILE, variable, 255)) {
        return "/usr/share/sai_2700.xml";
    }
//#if 0 //added during debbuging
#elif defined(CAVM_SAI)
    if (!strncmp("devType", variable, 255)) {
        return "1";
    } else if (!strncmp("initType", variable, 255)) {
        return "1";
    } else if (!strncmp("pipeLineNum", variable, 255)) {
        return "2";
    } else if (!strncmp("profileNum", variable, 255)) {
        return "1";
    } else if (!strncmp("performanceMode", variable, 255)) {
        return "0";
    } else if (!strncmp("mode", variable, 255)) {
        return "18";
    } else if (!strncmp("coreClkFreq", variable, 255)) {
        return "2";
    } else if (!strncmp("diag", variable, 255)) {
        return "0";
    } else if (!strncmp("hwCfgPath", variable, 255)) {
        return "";
    }
//#endif
#endif
    return NULL;
}
int profile_get_next_value(sai_switch_profile_id_t profile_id,
        const char** variable,
        const char** value)
{
    return -1;
}
const service_method_table_t services =
{
    profile_get_value,
    profile_get_next_value
};

int SaiQueryAndPopulateAPITables()
{
#ifdef SAI_BUILD
    sai_status_t rv;
    //Initialize switch API
    rv = sai_api_query(SAI_API_SWITCH, (void**)&sai_switch_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_switch_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving switch api table. Aborting init.");
        return -1;
    }
    //Initialize host intf API
    rv = sai_api_query(SAI_API_HOST_INTERFACE, (void**)&sai_hostif_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_hostif_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving switch api table. Aborting init.");
        return -1;
    }
    //Initialize vlan API
    rv = sai_api_query(SAI_API_VLAN, (void**)&sai_vlan_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_vlan_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving vlan api table. Aborting init.");
        return -1;
    }
    //Initialize fdb API
    rv = sai_api_query(SAI_API_FDB, (void**)&sai_fdb_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_fdb_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving fdb api table. Aborting init.");
        return -1;
    }
    //Initialize port API
    rv = sai_api_query(SAI_API_PORT, (void**)&sai_port_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_port_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving port api table. Aborting init.");
        return -1;
    }
    //Initialize VR API
    rv = sai_api_query(SAI_API_VIRTUAL_ROUTER, (void**)&sai_vr_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_vr_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving virtual router api table. Aborting init.");
        return -1;
    }
    //Initialize route API
    rv = sai_api_query(SAI_API_ROUTE, (void**)&sai_route_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_route_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving route api table. Aborting init.");
        return -1;
    }
    //Initialize neighbor API
    rv = sai_api_query(SAI_API_NEIGHBOR, (void**)&sai_nbr_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_nbr_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving neighbor api table. Aborting init.");
        return -1;
    }
    //Initialize next hop API
    rv = sai_api_query(SAI_API_NEXT_HOP, (void**)&sai_nh_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_nh_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving next hop api table. Aborting init.");
        return -1;
    }
    //Initialize next hop group API
    rv = sai_api_query(SAI_API_NEXT_HOP_GROUP, (void**)&sai_nh_grp_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_nh_grp_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving next hop group api table. Aborting init.");
        return -1;
    }
    //Initialize router interface API
    rv = sai_api_query(SAI_API_ROUTER_INTERFACE, (void**)&sai_rif_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_rif_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving router intf api table. Aborting init.");
        return -1;
    }
    //Initialize buffer interface API
    rv = sai_api_query(SAI_API_BUFFERS, (void**)&sai_buffer_api_tbl);
    if ((rv != SAI_STATUS_SUCCESS) || (sai_buffer_api_tbl == NULL)) {
         syslog(LOG_ERR, "Failure retrieving buffer api table. Aborting init.");
	 return -1; 
    }  
    rv = sai_api_query(SAI_API_ACL, (void**)&sai_acl_api_tbl); 
    if((rv != SAI_STATUS_SUCCESS) || (sai_acl_api_tbl == NULL)) {
	syslog(LOG_ERR, "Failure retrieving acl api table. Aborting init.");
   	return -1;
    }
    rv = sai_api_query(SAI_API_POLICER, (void**)&sai_policer_api_tbl);
    if((rv != SAI_STATUS_SUCCESS) || (sai_policer_api_tbl == NULL)) {
        syslog(LOG_ERR, "Failure retrieving policer api table. Aborting init.");
	return -1;
    }
#endif
    return 0;
}

static int SaiInitTrapId() 
{
#ifdef SAI_BUILD
    int i, cport, netDevType;
    sai_status_t rv;
    sai_attribute_t attr;
    /* Set traps to CPU */
    sai_hostif_trap_id_t sai_trap_ids[] = {
        SAI_HOSTIF_TRAP_ID_TTL_ERROR,
        SAI_HOSTIF_TRAP_ID_ARP_REQUEST,
        SAI_HOSTIF_TRAP_ID_ARP_RESPONSE,
        SAI_HOSTIF_TRAP_ID_DHCP,
        SAI_HOSTIF_TRAP_ID_LLDP,
        SAI_HOSTIF_TRAP_ID_LACP,
        SAI_HOSTIF_TRAP_ID_IPV6_NEIGHBOR_DISCOVERY,
#ifdef CAVM_SAI
        // xpSaiHostInterface.c defined here what they mean
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 1,
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 2,
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 3,
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 4,
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 5,
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 6,
        SAI_HOSTIF_TRAP_ID_ROUTER_CUSTOM_RANGE_BASE + 7,
#endif
    };
    /* Set netdev type per platform  */ 
#if defined(MLNX_SAI)
    netDevType = SAI_HOSTIF_TRAP_CHANNEL_L2_NETDEV;
#elif defined(CAVM_SAI)
    netDevType = SAI_HOSTIF_TRAP_CHANNEL_NETDEV;
#endif
    int traps = sizeof(sai_trap_ids)/sizeof(*sai_trap_ids);
        for (i = 0; i < traps; i++) {
            /* Setting PACKET ACTION Trap */
            attr.id = SAI_HOSTIF_TRAP_ATTR_PACKET_ACTION;
            attr.value.s32 = SAI_PACKET_ACTION_TRAP;
            rv = sai_hostif_api_tbl->set_trap_attribute(sai_trap_ids[i], &attr);
            if (rv != SAI_STATUS_SUCCESS)
              {
                syslog(LOG_ERR,"Failed to set packet action trap attribute %d, rv : 0x%x\n", sai_trap_ids[i], rv);
                return -1;
              } 
            /* Setting TRAP CHANNEL */
            attr.id = SAI_HOSTIF_TRAP_ATTR_TRAP_CHANNEL;
            attr.value.s32 = netDevType;
            rv = sai_hostif_api_tbl->set_trap_attribute(sai_trap_ids[i], &attr);
            if (rv != SAI_STATUS_SUCCESS)
              {
                syslog(LOG_ERR,"Failed to set channel trap attribute %d, rv : 0x%x\n", sai_trap_ids[i], rv);
                return -1;
              } 
        }
#endif
#ifdef MLNX_SAI // MLNX specific trap id's with different attribute    
    attr.id = SAI_HOSTIF_USER_DEFINED_TRAP_ATTR_TRAP_CHANNEL;        
    attr.value.s32 = netDevType;        
    rv = sai_hostif_api_tbl->set_user_defined_trap_attribute(SAI_HOSTIF_USER_DEFINED_TRAP_ID_ROUTER_MIN, &attr);        
    if (rv != SAI_STATUS_SUCCESS) {        
        syslog(LOG_ERR, "Failed to set trap channel as l2 netdev for packets matching route entry with trap action set. Aborting init");        
        return -1;        
    }        
    attr.id = SAI_HOSTIF_USER_DEFINED_TRAP_ATTR_TRAP_CHANNEL;        
    attr.value.s32 = netDevType;        
    rv = sai_hostif_api_tbl->set_user_defined_trap_attribute(SAI_HOSTIF_USER_DEFINED_TRAP_ID_NEIGH_MIN, &attr);        
    if (rv != SAI_STATUS_SUCCESS) {        
        syslog(LOG_ERR, "Failed to set trap channel as l2 netdev for packets matching neighbor entries with trap action set. Aborting init");        
        return -1;        
    }
#endif
    return SAI_STATUS_SUCCESS;
}

int SaiInit(int bootMode, int ifMapCount, ifMapInfo_t *ifMap, uint8_t *macAddr)
{
#ifdef SAI_BUILD
    int i, cport, netDevType;
    sai_status_t rv;
    sai_attribute_t attr;
    sai_object_id_t cpuPort;
    sai_switch_notification_t cb_list;
#ifdef CAVM_SAI
    char swHwAddr[16] = "as7512";
#else
    char swHwAddr[16] = "SaiSwitch";
#endif

    //Save switch mac addr locally
    memcpy(switchMacAddr, macAddr, MAC_ADDR_LEN);
    rv = sai_api_initialize(0, &services);
    if (rv !=  SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure initializing SAI switch APIs. Aborting init.");
        return -1;
    }
    //Populate API tables
    rv = SaiQueryAndPopulateAPITables();
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to query SAI API tables. Aborting init.");
        return -1;
    }
    //Initialize switch
    memset (&cb_list, 0, sizeof(sai_switch_notification_t));
    cb_list.on_switch_state_change = sai_switch_state_cb;
    cb_list.on_fdb_event = sai_fdb_cb;
    cb_list.on_port_state_change = sai_port_state_cb;
    cb_list.on_switch_shutdown_request = sai_switch_shutdown_cb;
    cb_list.on_port_event = sai_port_cb;
    cb_list.on_packet_event = sai_packet_cb;
    if (sai_switch_api_tbl->initialize_switch != NULL) {
        rv = sai_switch_api_tbl->initialize_switch (0, swHwAddr, NULL, &cb_list);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failure initalizing switch. Aborting init");
        }
    }
    //Connect to SDK
    rv = sai_switch_api_tbl->connect_switch(0, swHwAddr, &cb_list);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure connecting to SDK. Aborting init");
    }

#if 0
    // Added following attribute during debugging session
    attr.id = SAI_SWITCH_ATTR_SRC_MAC_ADDRESS;
    memcpy(attr.value.mac, switchMacAddr, 6);
    rv = sai_switch_api_tbl->set_switch_attribute(&attr);
    if (rv != SAI_STATUS_SUCCESS)
      {
        syslog(LOG_ERR, "Failed to set MAC address to switch 0x%x\n", rv);
        return -1;
      }
#endif
    // end of attribute
    //Cache max number of ports
    attr.id = SAI_SWITCH_ATTR_PORT_NUMBER;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure retrieving port id's. Aborting init");
    }    
    //Cache max number of ports
    maxSysPorts = attr.value.u32;
    syslog(LOG_INFO, "Retrieved port id's after doing portNum. Total ports count = %d\n", maxSysPorts);

    //Retrieve port object ids
    attr.id = SAI_SWITCH_ATTR_PORT_LIST;
    attr.value.objlist.count = MAX_NUM_PORTS;
    //port_list + 1 used below so that port numbers can be used to directly index array
    attr.value.objlist.list = port_list+1;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failure retrieving port id's after doing portList. Aborting init");
    }
    // Map front panel ports to linux interfaces
    rv = SaiCreateLinuxIfForAsicPorts(ifMapCount, ifMap);
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to map asic ports to linux interfaces");
        return -1;
    }
    // Disable all ports
    for (i = 1; i <= maxSysPorts; i++) {
        attr.id = SAI_PORT_ATTR_ADMIN_STATE;
        attr.value.booldata = false;
        rv = sai_port_api_tbl->set_port_attribute(port_list[i], &attr);
        if (rv != SAI_STATUS_SUCCESS) {
            syslog(LOG_ERR, "Failed to set port admin state for port %d", i);
            return -1;
        }
    }
    /* Retrieve global vr id */
    attr.id = SAI_SWITCH_ATTR_DEFAULT_VIRTUAL_ROUTER_ID;
    rv = sai_switch_api_tbl->get_switch_attribute(1, &attr);
    if (rv != SAI_STATUS_SUCCESS) {
        syslog(LOG_ERR, "Failed to retrieve global VR instance. Aborting init");
        return -1;
    }
    globalVrId = attr.value.oid;
    //Initialize port data structure
    SaiPortInit(ifMapCount, ifMap);

    /* Init Trap ID for System */
    if (SaiInitTrapId() == SAI_STATUS_SUCCESS) {
        syslog(LOG_INFO, "Asicd SAI trapid  initialization complete !");
    } else {
        syslog(LOG_ERR, "Asicd SAI Init Failed!!!!");
    }
    
    /* init CoPP feature */
    if (SaiCoPPInit() == SAI_STATUS_SUCCESS) {
	syslog(LOG_INFO, "Asicd COPP initialization done.!");
    } else {
	syslog(LOG_ERR, "Copp SAI init failed.");
    }
    if (SaiCoPPConfig() == SAI_STATUS_SUCCESS) {
	syslog(LOG_INFO, "Asicd CoPP config done.!");
    } else {
	syslog(LOG_ERR, "CoPP SAI config failed.");
	syslog(LOG_ERR, "Asicd SAI Init Failed!!!!");
    }
    syslog(LOG_INFO, "Asicd SAI initialization complete !");
#endif
    return 0;
}

int SaiDeinit(int cacheSwState)
{
#ifdef SAI_BUILD
    int rv;

    if (cacheSwState) {
        //Code to enable warmboot state sync
    }
    rv = SaiDeleteLinuxIfForAsicPorts(ALL_INTERFACES, NULL);
    if (rv < 0) {
        syslog(LOG_ERR, "Failed to delete mapping of asic ports to linux interfaces");
        return -1;
    }
#endif
    return 0;
}

int SaiCreateLag(int hashType, int portCount, int *ports)
{
    return 0;
}

int SaiDeleteLag(int lagId)
{
    return 0;
}

int SaiUpdateLag(int lagId, int hashType, int oldPortCount, int *oldPorts, int portCount, int *ports)
{
    return 0;
}

int SaiRestoreLagDB()
{
    return 0;
}

void SaiDevShell(void)
{

}
