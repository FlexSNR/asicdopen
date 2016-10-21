//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//

package publisher

import (
	"asicd/asicdCommonDefs"
	"github.com/op/go-nanomsg"
	"syscall"
	"utils/logging"
)

const (
	NOTIFICATION_BUFFER_SIZE    = 100
	PUB_SOCKET_SEND_BUFFER_SIZE = 1024 * 1024
)

type PubChannels struct {
	All  chan []byte
	Ribd chan []byte
}

type PublisherInfo struct {
	Logger  *logging.Writer
	PubChan *PubChannels
	All     *nanomsg.PubSocket
	Ribd    *nanomsg.PubSocket
}

func (p *PublisherInfo) CreateAndBindPubSock(socketAddr string, sockBufSize int64) *nanomsg.PubSocket {
	pubSock, err := nanomsg.NewPubSocket()
	if err != nil {
		p.Logger.Err("Failed to open publisher socket")
	}
	_, err = pubSock.Bind(socketAddr)
	if err != nil {
		p.Logger.Err("Failed to bind publisher socket")
	}
	err = pubSock.SetSendBuffer(sockBufSize)
	if err != nil {
		p.Logger.Err("Failed to set send buffer size for pub socket")
	}
	return pubSock
}

func (p *PublisherInfo) PublishEvents() {
	for {
		var msg []byte
		//Drain notification channels and publish event
		select {
		case msg = <-p.PubChan.All:
			_, rv := p.All.Send(msg, nanomsg.DontWait)
			if rv == syscall.EAGAIN {
				p.Logger.Err("Failed to publish event to all clients")
			}
		case msg = <-p.PubChan.Ribd:
			_, rv := p.Ribd.Send(msg, nanomsg.DontWait)
			if rv == syscall.EAGAIN {
				p.Logger.Err("Failed to publish event to ribd client")
			}
		}
	}
}

func NewPublisher() *PublisherInfo {
	return new(PublisherInfo)
}

func (p *PublisherInfo) InitPublisher(logger *logging.Writer) *PubChannels {
	pubChan := new(PubChannels)
	pubChan.All = make(chan []byte, NOTIFICATION_BUFFER_SIZE)
	pubChan.Ribd = make(chan []byte, NOTIFICATION_BUFFER_SIZE)
	p.Logger = logger
	p.PubChan = pubChan
	p.All = p.CreateAndBindPubSock(asicdCommonDefs.PUB_SOCKET_ADDR, PUB_SOCKET_SEND_BUFFER_SIZE)
	p.Ribd = p.CreateAndBindPubSock(asicdCommonDefs.PUB_SOCKET_RIBD_CLIENT_ADDR, PUB_SOCKET_SEND_BUFFER_SIZE)
	go p.PublishEvents()
	return pubChan
}

func (p *PublisherInfo) DeinitPublisher() {
	//Close nanomsg sockets
	if err := p.All.Close(); err != nil {
		p.Logger.Err("Failed to close nano msg publisher socket for all clients")
	}
	if err := p.Ribd.Close(); err != nil {
		p.Logger.Err("Failed to close nano msg publisher socket for ribd client")
	}
	return
}
