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
PLUGIN_MGR_DIR = $(SR_CODE_BASE)/snaproute/src/asicd/pluginManager
OPENNSL_DRIVER_DIR = $(PLUGIN_MGR_DIR)/opennsl/
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
endif

#Targets
all:ipc exe

ipc:
	$(IPC_GEN_CMD) -r --gen go -out $(GENERATED_IPC) $(IPC_SRCS)

exe:
	bash makeSymLinks.sh $(SAI_TARGET) $(OPENNSL_TARGET) $(BCMSDK_TARGET)
	go build -o $(DESTDIR)/$(COMP_NAME) -ldflags="$(GOLDFLAGS):$(PLUGIN_MGR_DIR)/opennsl:$(PLUGIN_MGR_DIR)/sai/mlnx/libs:$(PLUGIN_MGR_DIR)/sai/bfoot/libs:$(PLUGIN_MGR_DIR)/bcmsdk"

install:
	$(MKDIR) $(PARAMSDIR)
	install params/asicdConf.json $(PARAMSDIR)/
ifeq ($(SAI_BUILD), true)
	$(CP_R) $(SAI_LIBS) $(DESTDIR)/sharedlib
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
