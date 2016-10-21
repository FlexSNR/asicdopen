#include <stdio.h>
#include <string.h>
#include <syslog.h>
#include <sai.h>
#include <saitypes.h>
#include <saibuffer.h>
#include "_cgo_export.h"
#include "../pluginCommon/pluginCommon.h"

sai_buffer_api_t* sai_buffer_api_tbl = NULL;

sai_ingress_priority_group_stat_counter_t bufferPortStatTypeEnumMap[bufferGlobalStatTypesMax];

int SaiInitBufferGlobalStateDB()
{
#ifdef MLNX_SAI
	syslog(LOG_DEBUG, "Called init buffer global stat ");
#endif
	return;
}

void SaiPopulateBufferGlobalMap() {
	/*
	 * Currently Only ingress stats are supported by mlnx
	 */
	bufferPortStatTypeEnumMap[BufferStat] = -1;
	bufferPortStatTypeEnumMap[EgressBufferStat] = -1;
	bufferPortStatTypeEnumMap[IngressBufferStat] = SAI_INGRESS_PRIORITY_GROUP_STAT_CURR_OCCUPANCY_BYTES;
}

int SaiUpdateBufferGlobalStatDB(int startId, int endId) 
{
	int index, stat_type;
	uint64_t counter_list[bufferGlobalStatTypesMax] = {0};
	bufferGlobalState info;
	sai_object_id_t default_pg_id = 0;
	info.deviceId = default_pg_id;
	sai_status_t rv;
	sai_ingress_priority_group_stat_counter_t statTypeList[] = {
		SAI_INGRESS_PRIORITY_GROUP_STAT_CURR_OCCUPANCY_BYTES,
	};	

#ifdef MLNX_SAI
	stat_type = bufferPortStatTypeEnumMap[index];
	if(stat_type != -1) { 
		syslog(LOG_DEBUG, "BUFFERGLOBAL: Get stats for SAI ");
		/*
		rv = sai_buffer_api_tbl->get_ingress_priority_group_stats(default_pg_id,
				statTypeList,sizeof(statTypeList)/sizeof(sai_ingress_priority_group_stat_counter_t), (uint64_t*)counter_list);
		if(rv != SAI_STATUS_SUCCESS) { 
			syslog(LOG_ERR, "Failed to retrieve buffer global stats for index %d", index);
			return -1;

		} */

	}
	info.stats[index] = counter_list[0];
	syslog(LOG_DEBUG, "BUFFERGLOBAL: callback update bufferglobal stat.");
	SaiIntfUpdateBufferGlobalStateDB(&info);
#endif
		return 0;
}
