#include "saiAcl.h"

CoppData_t coppStruct[CoppClassCount];
CoppGlobalData cgData;
sai_acl_api_t *sai_acl_api_tbl = NULL;
sai_policer_api_t *sai_policer_api_tbl = NULL;
extern sai_port_api_t* sai_port_api_tbl; 
extern sai_switch_api_t* sai_switch_api_tbl;

/*Initialise COPP structs */
int SaiCoPPInit() {
	cgData.table_id = 0;
	int status;
	int index;
#ifdef SAI_BUILD
	/* Generate all ingress attributes. */
	sai_status_t rv;
	sai_attribute_t sw_attr;

	sw_attr.id = SAI_SWITCH_ATTR_CPU_PORT;
	rv = sai_switch_api_tbl->get_switch_attribute(1, &sw_attr);
	if (rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to get CPU port it. Abort init");
		return -1;
	}
	cgData.cpu_port = sw_attr.value.oid;


	/* Get queue list */
	status = populateCpuQueueList();
	if(status != 0) {
		syslog(LOG_ERR, "CoPP: Failed to get CPU queues. CoPP will not be configured.");
		return -1;
	}


	sai_attribute_t attr[COPP_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR]; 
	int i = 0;

	attr[0].id = SAI_ACL_TABLE_ATTR_STAGE;
	attr[0].value.s32 = SAI_ACL_STAGE_INGRESS; 
	attr[1].id = SAI_ACL_TABLE_ATTR_PRIORITY;
	attr[1].value.s32 = SAI_SWITCH_ATTR_ACL_TABLE_MINIMUM_PRIORITY;
	attr[2].id = SAI_ACL_TABLE_ATTR_SIZE;
	attr[2].value.s32 = COPP_ACL_TABLE_SIZE;


	for (i=ACL_DEFAULT_TABLE_ATTR; i < COPP_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR; i++) {
		index = i - ACL_DEFAULT_TABLE_ATTR;
		attr[i].id = CoppAclTblAttr[index].id;
		attr[i].value.booldata = true;
	} 

	/* create acl table */
	rv = sai_acl_api_tbl->create_acl_table(&cgData.table_id, COPP_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR , attr);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "CoPP: Failed to create ACL table. CoPP will not work err %x", rv);
		return -1;
	}
	syslog(LOG_DEBUG, "COPP: acl table created with id %lld ", (long long int)cgData.table_id);


#endif
	return 0;
}

/*Confgure COPP for each protocol */
int SaiCoPPConfig() {
	int i;
	int status;
	sai_object_id_t policer;
	if(cgData.table_id == 0) {
		syslog(LOG_ERR, "CoPP: ACl table is not created. CoPP will not be configured. ");
		return -1;
	}
#ifdef SAI_BUILD
	// create policer
	initPolicerRates();
	status = SaiCreatePolicer(i,
			coppStruct[0].policer_cir,
			20, 
			coppStruct[i].policer_pbs,
			&policer);
	if(status != 0) {
		syslog(LOG_ERR, "COPP: Failed to create policer for index %d. CoPP wont be configured.",i);
		return -1;
	}

	for(i =0; i < CoppClassCount; i++) {
		/*status = SaiCreatePolicer(i,
		  coppStruct[i].policer_cir,
		  20, // Current max value for cbs is 30 as per mlnx implementation
		  30, //coppStruct[i].policer_pbs,
		  &policer);
		  if(status != 0) {
		  syslog(LOG_ERR, "COPP: Failed to create policer for index %d. CoPP wont be configured.",i);
		  return -1;
		  } */

		status = SaiAddAclEntry(i);
		if(status != 0) {
			syslog(LOG_ERR, "COPP: Failed to add ACl entry for index %d ", i);
			return -1;
		}

	}
#endif
}

int SaiAddAclEntry(int index) {
#ifdef SAI_BUILD
	int rv;
	switch(index) {
		case  	     CoppArpUC:
		case         CoppArpMC:
			//syslog(LOG_DEBUG, "COPP: configure Arp UC");
			rv = SaiConfigArp(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for arp . Exiting");
				return rv;
			}
			break;

		case         CoppBgp:
			//syslog(LOG_DEBUG, "COPP: configure BGP");
			rv = SaiConfigBgp(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for bgp . Exiting");
				return rv;
			}

			break;

		case         CoppIcmpV4UC:
			//syslog(LOG_DEBUG, "COPP: configure ICMP v4");
			rv = SaiConfigIcmpv4UC(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for ICMPv4UC . Exiting");
				return rv;
			}


			break;

		case         CoppIcmpV4BC:
			//syslog(LOG_DEBUG, "COPP: configure  CoppIcmpV4BC");
			rv = SaiConfigIcmpv4BC(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for ICMPv4BC . Exiting");
				return rv;
			}

			break;

		case        CoppStp:
			//syslog(LOG_DEBUG, "COPP: configure stp");
			rv = SaiConfigStp(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for STP . Exiting");
				return rv;
			}

			break;

		case         CoppLacp:
			//syslog(LOG_DEBUG, "COPP: configure lacp");
			rv = SaiConfigLacp(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for LACP . Exiting");
				return rv;
			}

			break;

		case         CoppBfd:
			//syslog(LOG_DEBUG, "COPP: configure Arp bfd");
			rv = SaiConfigBfd(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for bfd . Exiting");
				return rv;
			}

			break;

		case        CoppIcmpv6:
			//syslog(LOG_DEBUG, "COPP: configure icmp v6");
			//Platform does not support IPV6 src and dest IPs
			/*
			rv = SaiConfigIcmpv6(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for Icmpv6 . Exiting");
				return rv;
			}*/

			break;


		case         CoppLldp:
			//syslog(LOG_DEBUG, "COPP: configure lldp");
			rv = SaiConfigLldp(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for LLDP. Exiting");
				return rv;
			}

			break;
	#if 0
		case        CoppSsh:
			//syslog(LOG_DEBUG, "COPP: configure ssh");
			rv = SaiConfigSsh(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for ssh . Exiting");
				return rv;
			}

			break;

		case         CoppHttp:
			//syslog(LOG_DEBUG, "COPP: configure http");
			rv = SaiConfigHttp(index);
			if(rv != 0) {
				syslog(LOG_ERR, "COPP: Failed to config copp for Http . Exiting");
				return rv;
			}

			break;
#endif
	}
	return 0;
#endif
}

int populateCpuQueueList() {
#ifdef SAI_BUILD
	sai_status_t rv;
	int no_of_queues_per_port = 0;
	sai_attribute_t sai_attr; 
	sai_attr.id = SAI_PORT_ATTR_QOS_NUMBER_OF_QUEUES;
	sai_attr.value.u32 = 0;

	rv = sai_port_api_tbl->get_port_attribute(cgData.cpu_port, 1,&sai_attr);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to get cpu # queues %d ", rv);
		return -1;
	}

	no_of_queues_per_port = sai_attr.value.u32; 
	cgData.cpu_queue_count = no_of_queues_per_port;
	syslog(LOG_DEBUG, "COPP: Total number of queues for cpu %d ", no_of_queues_per_port);

	sai_attr.id = SAI_PORT_ATTR_QOS_QUEUE_LIST;
	sai_attr.value.objlist.count = no_of_queues_per_port;
	sai_attr.value.objlist.list = calloc(no_of_queues_per_port, sizeof(sai_object_id_t));

	/* Get queue list */
	rv = sai_port_api_tbl->get_port_attribute(cgData.cpu_port, 1,&sai_attr);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to get cpu queue list %d", rv);
		return -1;
	}   

	cgData.sai_cpu_queue_list.count = sai_attr.value.objlist.count;

	cgData.sai_cpu_queue_list.list = calloc(cgData.sai_cpu_queue_list.count, sizeof(sai_object_id_t));

	memcpy((char *)cgData.sai_cpu_queue_list.list, (char *)sai_attr.value.objlist.list, sizeof(sai_object_id_t) * cgData.sai_cpu_queue_list.count);
#endif
	return 0;
}


int initPolicerRates() {
#ifdef SAI_BUILD
	int index;

	//arp UC
	coppStruct[0].policer_cir=COPP_POLICER_LOW_BW;
	coppStruct[0].policer_cbs=COPP_POLICER_LOW_BURST;
	coppStruct[0].policer_pbs=COPP_POLICER_LOW_BURST;

	// arp MC
	coppStruct[1].policer_cir=COPP_POLICER_LOW_BW;
	coppStruct[1].policer_cbs=COPP_POLICER_LOW_BURST;
	coppStruct[1].policer_pbs=COPP_POLICER_LOW_BURST;

	//BGP
	coppStruct[2].policer_cir=COPP_POLICER_MED_BW;
	coppStruct[2].policer_cbs=COPP_POLICER_MED_BURST;
	coppStruct[2].policer_pbs=COPP_POLICER_MED_BURST;

	//Icmpv4UC
	coppStruct[3].policer_cir=COPP_POLICER_MED_BW;
	coppStruct[3].policer_cbs=COPP_POLICER_MED_BURST;
	coppStruct[3].policer_pbs=COPP_POLICER_MED_BURST;

	//Icmpv4BC
	coppStruct[4].policer_cir=COPP_POLICER_MED_BW;
	coppStruct[4].policer_cbs=COPP_POLICER_MED_BURST;
	coppStruct[4].policer_pbs=COPP_POLICER_MED_BURST;

	//STP
	coppStruct[5].policer_cir=COPP_POLICER_HIGH_BW;
	coppStruct[5].policer_cbs=COPP_POLICER_HIGH_BURST;
	coppStruct[5].policer_pbs=COPP_POLICER_HIGH_BURST;

	//LACP
	coppStruct[6].policer_cir=COPP_POLICER_LOW_BW;
	coppStruct[6].policer_cbs=COPP_POLICER_LOW_BURST;
	coppStruct[6].policer_pbs=COPP_POLICER_LOW_BURST;

	//BFD
	coppStruct[7].policer_cir=COPP_POLICER_HIGH_BW;
	coppStruct[7].policer_cbs=COPP_POLICER_HIGH_BURST;
	coppStruct[7].policer_pbs=COPP_POLICER_HIGH_BURST;

	//ICMPv6
	coppStruct[8].policer_cir=COPP_POLICER_MED_BW;
	coppStruct[8].policer_cbs=COPP_POLICER_MED_BURST;
	coppStruct[8].policer_pbs=COPP_POLICER_MED_BURST;

	//LLDP
	coppStruct[9].policer_cir=COPP_POLICER_LOW_BW;
	coppStruct[9].policer_cbs=COPP_POLICER_LOW_BURST;
	coppStruct[9].policer_pbs=COPP_POLICER_LOW_BURST;

	//SSH
	coppStruct[10].policer_cir=COPP_POLICER_LOW_BW;
	coppStruct[10].policer_cbs=COPP_POLICER_LOW_BURST;
	coppStruct[10].policer_pbs=COPP_POLICER_LOW_BURST;

	//HTTP
	coppStruct[11].policer_cir=COPP_POLICER_LOW_BW;
	coppStruct[11].policer_cbs=COPP_POLICER_LOW_BURST;
	coppStruct[11].policer_pbs=COPP_POLICER_LOW_BURST;

#endif
}

int SaiCreatePolicer(int index, 
		uint64_t policer_cir, 
		uint64_t policer_cbs, 
		uint64_t policer_pbs, 
		sai_object_id_t *policer_id) {
#ifdef SAI_BUILD
	uint64_t policer;
	sai_object_id_t polid;
	int max_policer_attr = 7;
	sai_attribute_t policer_attr[max_policer_attr];
	sai_status_t rv;
	int attrIdx = 0;
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_METER_TYPE;
	policer_attr[attrIdx].value.s32 = SAI_METER_TYPE_PACKETS;
	attrIdx++;
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_MODE;
	policer_attr[attrIdx].value.s32 = SAI_POLICER_MODE_Sr_TCM;
	attrIdx++;
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_COLOR_SOURCE;
	policer_attr[attrIdx].value.s32 = SAI_POLICER_COLOR_SOURCE_BLIND;
	attrIdx++;
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_CBS;
	policer_attr[attrIdx].value.u64 = policer_cbs;
	attrIdx++;
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_CIR;
	policer_attr[attrIdx].value.u64 = policer_cir;
	attrIdx++;
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_PBS;
	policer_attr[attrIdx].value.u64 = policer_pbs;
	attrIdx++; 
	policer_attr[attrIdx].id = SAI_POLICER_ATTR_RED_PACKET_ACTION;
	policer_attr[attrIdx].value.s32 = SAI_PACKET_ACTION_DROP;


	rv = sai_policer_api_tbl->create_policer(&(coppStruct[0].policer_id), max_policer_attr, policer_attr);
	if (rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create policer for index %d ", index);
		return rv;
	}
	//coppStruct[index].policer_id = polid;
	//policer = *policer_id;

	//syslog(LOG_DEBUG, "COPP : Class %d policer id  %x ", index, coppStruct[0].policer_id);
#endif
	return 0;
}

int SaiConfigArp(int index) {
#ifdef SAI_BUILD
	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int attrNum = 5-1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * attrNum);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_ETHER_TYPE;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_ARP_ETHER_TYPE;
	attr_list[id].value.aclfield.mask.u32 = COPP_L2_ETHER_TYPE_MASK;
	id++;
	
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];


	//TODO Need to identify unicast/multicast packets.
	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, attrNum, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for Arp UC %d", rv);

	}

	free(attr_list);
#endif
	return 0;
}

int SaiConfigBgp(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int attrNum = 7-1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * attrNum);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_L4_DST_PORT;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_L4_PORT_BGP;
	attr_list[id].value.aclfield.mask.u32 = COPP_L4_PORT_MASK;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_IP_PROTOCOL;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_IP_PROTOCOL_IP_NUMBER_TCP;
	attr_list[id].value.aclfield.mask.u32 = COPP_IP_PROTOCOL_IP_NUMBER_MASK;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_DST_IP;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_DST_IP_LOCAL_DATA;
	attr_list[id].value.aclfield.mask.u32 = COPP_DST_IP_LOCAL_MASK;
	id++;

	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];


	//TODO Need to identify unicast/multicast packets.
	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, attrNum, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for BGP index %d err %d", index, rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;
}

int SaiConfigIcmpv4UC(int index) {
#ifdef SAI_BUILD


	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 6-1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_IP_PROTOCOL;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_IP_PROTOCOL_IPV4_NUMBER_ICMP;
	attr_list[id].value.aclfield.mask.u32 = COPP_IP_PROTOCOL_IP_NUMBER_MASK;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_DST_IP;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_DST_IP_LOCAL_DATA;
	attr_list[id].value.aclfield.mask.u32 = COPP_DST_IP_LOCAL_MASK;
	id++;

	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];


	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for ICMP v4 %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;

}

int SaiConfigIcmpv4BC(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 6-1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_IP_PROTOCOL;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_IP_PROTOCOL_IPV4_NUMBER_ICMP;
	attr_list[id].value.aclfield.mask.u32 = COPP_IP_PROTOCOL_IP_NUMBER_MASK;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_DST_IP;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= inet_network(COPP_L3_IPV4_BROADCAST_ADDR);
	attr_list[id].value.aclfield.mask.u32 = inet_network(COPP_L3_IPV4_ADDR_MASK);
	id++;

	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];


	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for ICMPv4 bc %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;

}

int SaiConfigStp(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5-1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_ETHER_TYPE;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_STP_ETHER_TYPE;
	attr_list[id].value.aclfield.mask.u32 = COPP_L2_ETHER_TYPE_MASK;
	id++;
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];


	//TODO Need to identify unicast/multicast packets.
	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for STP %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;
}

int SaiConfigLacp(int index) {
#ifdef SAI_BUILD


	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5-1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_ETHER_TYPE;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_LACP_ETHER_TYPE;
	attr_list[id].value.aclfield.mask.u32 = COPP_L2_ETHER_TYPE_MASK;
	id++;
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];

	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for LACP %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);
#endif
	return 0;
}


int SaiConfigBfd(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5 -1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_L4_DST_PORT;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_L4_PORT_BFD;
	attr_list[id].value.aclfield.mask.u32 = COPP_L4_PORT_MASK;
	id++;
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];

	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for BFD %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;
}

int SaiConfigLldp(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5 -1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_ETHER_TYPE;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_LLDP_ETHER_TYPE;
	attr_list[id].value.aclfield.mask.u32 = COPP_L2_ETHER_TYPE_MASK;
	id++;
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];

	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for LLDP %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;
}

int SaiConfigIcmpv6(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5 -1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_SRC_IPv6;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_IP_PROTOCOL_IPV6_NUMBER_ICMP;
	attr_list[id].value.aclfield.mask.u32 =  COPP_IP_PROTOCOL_IP_NUMBER_MASK;
	id++;

	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];

	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for Icmpv6 %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;
}

int SaiConfigSsh(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5 -1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_L4_DST_PORT;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_L4_PORT_SSH;
	attr_list[id].value.aclfield.mask.u32 = COPP_L4_PORT_MASK;
	id++;
	
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];

	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for SSH %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;
}

int SaiConfigHttp(int index) {
#ifdef SAI_BUILD

	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;
	int numAttr = 5 -1;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * numAttr);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_L4_DST_PORT;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= COPP_L4_PORT_HTTP;
	attr_list[id].value.aclfield.mask.u32 = COPP_L4_PORT_MASK;
	id++;
	/*
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_POLICER;
	attr_list[id].value.aclaction.parameter.oid = coppStruct[0].policer_id;
	id++; */
	//syslog(LOG_INFO, "CoPP: Total # queues %d ", cgData.sai_cpu_queue_list.count);
	if(cgData.sai_cpu_queue_list.list == NULL) {
		syslog(LOG_ERR, "CoPP : cpu_queue list is null.");
	}
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_ACTION_SET_TC;
	attr_list[id].value.aclaction.parameter.u8 = cgData.sai_cpu_queue_list.list[SAI_QOS_PRIORITY_1];

	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, numAttr, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for HTTP %d", rv);
		free(attr_list);
		return -1;
	}

	free(attr_list);

#endif
	return 0;

}
/** COPP stats API */

