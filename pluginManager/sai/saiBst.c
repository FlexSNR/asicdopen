#include <stdio.h>
#include <string.h>
#include <syslog.h>

#include <sai.h>
#include <saibuffer.h>
#include <saitypes.h>

#include "_cgo_export.h"
#include "../pluginCommon/pluginCommon.h"

sai_buffer_api_t* sai_buffer_api_tbl = NULL;

sai_ingress_priority_group_stat_counter_t bufferPortStatTypeEnumMap[bufferGlobalStatTypesMax];

int SaiInitBufferGlobalStateDB()
{
	return 0;
}

void SaiPopulateBufferGlobalMap()
{
}

int SaiUpdateBufferGlobalStatDB(int startId, int endId)
{
		return 0;
}
