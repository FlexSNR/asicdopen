#include "saiAcl.h"

extern sai_acl_api_t *sai_acl_api_tbl;
extern sai_port_api_t* sai_port_api_tbl; 
CoppGlobalData cgData;
/*Initialise ACL structs */
int SaiAclInit() {
	int status;
	int index;
#ifdef SAI_BUILD
	/* Generate all ingress attributes. */
	cgData.table_id = 0;
	sai_status_t rv;
	sai_attribute_t sw_attr;


	sai_attribute_t attr[ACL_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR]; 
	int i = 0;

	attr[0].id = SAI_ACL_TABLE_ATTR_STAGE;
	attr[0].value.s32 = SAI_ACL_STAGE_INGRESS; 
	attr[1].id = SAI_ACL_TABLE_ATTR_PRIORITY;
	attr[1].value.s32 = SAI_SWITCH_ATTR_ACL_TABLE_MINIMUM_PRIORITY;
	attr[2].id = SAI_ACL_TABLE_ATTR_SIZE;
	attr[2].value.s32 = ACL_TABLE_SIZE;


	for (i=ACL_DEFAULT_TABLE_ATTR; i < ACL_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR; i++) {
		index = i - ACL_DEFAULT_TABLE_ATTR;
		attr[i].id = AclIngressTblAttr[index].id;
		attr[i].value.booldata = true;
	} 

	/* create acl table */
	rv = sai_acl_api_tbl->create_acl_table(&cgData.table_id, ACL_TABLE_ATTR + ACL_DEFAULT_TABLE_ATTR , attr);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "CoPP: Failed to create ACL table. CoPP will not work err %x", rv);
		return -1;
	}
	syslog(LOG_DEBUG, "ACL: acl table created with id %lld ", (long long int)cgData.table_id);


#endif
	return 0;
}
int SaiProcessAcl() {}

int SaiConfigEgressAcl(uint32_t src_port, uint32_t dst_port) {
#ifdef SAI_BUILD
	sai_attribute_t *attr_list = NULL;
	sai_object_id_t acl_entry;
	sai_status_t rv;
	int id = 0;

	attr_list = (sai_attribute_t *)malloc(sizeof(sai_attribute_t) * 5);
	attr_list[id].id = SAI_ACL_ENTRY_ATTR_TABLE_ID;
	attr_list[id].value.oid = cgData.table_id;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PRIORITY;
	attr_list[id].value.u32 = 5;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_IN_PORT;
	attr_list[id].value.aclfield.enable = true;
	attr_list[id].value.aclfield.data.u32= src_port;
	id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_FIELD_OUT_PORT;
        attr_list[id].value.aclfield.enable = true;
        attr_list[id].value.aclfield.data.u32= dst_port;
        id++;

	attr_list[id].id = SAI_ACL_ENTRY_ATTR_PACKET_ACTION;
	attr_list[id].value.aclaction.parameter.u8 = SAI_PACKET_ACTION_DROP;


	rv = sai_acl_api_tbl->create_acl_entry(&acl_entry, 5, attr_list);
	if(rv != SAI_STATUS_SUCCESS) {
		syslog(LOG_ERR, "COPP: Failed to create acl entry for mlag  %d", rv);

	}

	free(attr_list);
#endif
	return 0;
}
