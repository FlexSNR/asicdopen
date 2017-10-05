#CMD DEFS
MKDIR = mkdir -p
RMFORCE = rm -rf
CP_R = cp -r
SED_I = sed -i

#Component name
COMP_NAME = asicd

#Set default paths
PARAMSDIR = $(DESTDIR)/params
DESTDIR = $(SR_CODE_BASE)/snaproute/src/out/bin
BCMDIR = $(SR_CODE_BASE)/snaproute/src/bin-AsicdBcm
PLUGIN_MGR_DIR = $(SR_CODE_BASE)/snaproute/src/asicd/pluginManager
DRIVER_DIR = $(BCMDIR)/$(BUILD_TARGET)/lib
OPENNSL_DRIVER_DIR = $(DRIVER_DIR)
OPENNSL_HEADER_DIR = $(BCMDIR)/$(BUILD_TARGET)/opennsl/include
SAI_DRIVER_DIR = $(DRIVER_DIR)
SAI_HEADER_DIR = $(BCMDIR)/$(BUILD_TARGET)/sai/include/sai
SR_VENDORS = $(SR_CODE_BASE)/snaproute/src/vendors

#Set default build target
SAI_TARGETS = mlnx cavm bfoot
DOCKER_TARGET = docker

#IPC related vars
GENERATED_IPC = $(SR_CODE_BASE)/generated/src
IPC_GEN_CMD = thrift
IPC_SRCS = rpc/asicd.thrift
IPC_SVC_NAME = asicd

#Go linker flags
GOLDFLAGS = -r /opt/flexswitch/sharedlib

#CGO CFLAGS, LDFLAGS
export CGO_LDFLAGS=-lnanomsg
export CGO_LDFLAGS+=-L$(PLUGIN_MGR_DIR)/pluginCommon/utils -lhash
export CGO_CFLAGS+=-I$(PLUGIN_MGR_DIR)/pluginCommon/utils
export CGO_CFLAGS+=-I$(PLUGIN_MGR_DIR)/pluginCommon

ifeq ($(BUILD_TARGET), mlnx)
export CGO_CFLAGS+=-I$(SR_VENDORS)/sai/mlnx -D MLNX_SAI -D SAI_BUILD
export CGO_LDFLAGS+=-L$(PLUGIN_MGR_DIR)/sai/mlnx/sn2700/lib -lsai -lsw_rm_int -lsw_rm -lsxapi -lsxcomp -lsxdemadparser -lsxdemad -lsxdev -lsxdreg_access -lsxlog -lsxnet -lsxutils -lsxgenutils -lsxcom
	SAI_LIBS = $(PLUGIN_MGR_DIR)/sai/mlnx/sn2700/lib/*
	SAI_TARGET = mlnx
	SAI_BUILD = true
else ifeq ($(BUILD_TARGET), cavm)
export CGO_CFLAGS+=-I$(SR_VENDORS)/sai/cavm/ -D CAVM_SAI -D SAI_BUILD
export CGO_LDFLAGS+=-L$(PLUGIN_MGR_DIR)/sai/cavm/libs -laapl-BSD -lXpSaiAdapter
	SAI_LIBS = $(PLUGIN_MGR_DIR)/sai/cavm/libs/*
	SAI_TARGET = cavm
	SAI_BUILD = true
else ifeq ($(BUILD_TARGET), mlnx)
export CGO_CFLAGS+=-I$(SR_VENDORS)/sai/mlnx
	SAI_TARGET = none
	SAI_BUILD = false
else ifeq ($(BUILD_TARGET), ingrasys_s9100)
export CGO_CFLAGS+= -I$(OPENNSL_HEADER_DIR) -DINCLUDE_L3
export CGO_LDFLAGS+= -L$(OPENNSL_DRIVER_DIR) -lopennsl
export CGO_CFLAGS+= -I$(SAI_HEADER_DIR) -DSAI_BUILD
export CGO_LDFLAGS+= -L$(SAI_DRIVER_DIR) -lsai
	SAI_TARGET = none
	SAI_LIBS = $(DRIVER_DIR)/*
	SAI_BUILD = true
	#OPENNSL_BUILD = true
endif

#Targets
all:ipc exe

ipc:
	$(IPC_GEN_CMD) -r --gen go -out $(GENERATED_IPC) $(IPC_SRCS)

exe:
	bash makeSymLinks.sh $(SAI_TARGET) $(OPENNSL_TARGET) $(BCMSDK_TARGET)
	go build -o $(DESTDIR)/$(COMP_NAME) -ldflags="$(GOLDFLAGS):$(OPENNSL_DRIVER_DIR):$(PLUGIN_MGR_DIR)/opennsl:$(PLUGIN_MGR_DIR)/sai/mlnx/libs:$(PLUGIN_MGR_DIR)/sai/bfoot/libs:$(PLUGIN_MGR_DIR)/bcmsdk"

install:
	$(MKDIR) $(PARAMSDIR)
	install params/asicdConf.json $(PARAMSDIR)/
ifeq ($(SAI_BUILD), true)
	$(CP_R) $(BCMDIR)/$(BUILD_TARGET)/kmod/*.ko $(DESTDIR)/kmod/
	$(CP_R) $(SAI_LIBS) $(DESTDIR)/sharedlib
endif
ifeq ($(OPENNSL_BUILD), true)
	$(CP_R) $(BCMDIR)/$(BUILD_TARGET)/kmod/*.ko $(DESTDIR)/kmod/
	$(CP_R) $(BCMDIR)/$(BUILD_TARGET)/lib/libopennsl.so.1 $(DESTDIR)/sharedlib/
endif
	$(CP_R) $(PLUGIN_MGR_DIR)/pluginCommon/utils/libhash.so.1 $(DESTDIR)/sharedlib

guard:
ifndef SR_CODE_BASE
	$(error SR_CODE_BASE is not set)
endif

clean:guard
	$(RMFORCE) $(COMP_NAME)
	$(RMFORCE) $(GENERATED_IPC)/$(IPC_SVC_NAME)
	./cleanUp.sh
