RM = rm
MKDIR = mkdir -p
RMFORCE = rm -rf
DESTDIR=$(SR_CODE_BASE)/snaproute/src/out/bin
COMP_NAME = asicdUT

all: exe

exe:
	go build -o $(DESTDIR)/$(COMP_NAME)

guard:
ifndef SR_CODE_BASE
	$(error SR_CODE_BASE is not set)
endif
