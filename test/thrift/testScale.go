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

package testThrift

import (
	"asicdInt"
	"asicdServices"
	"fmt"
	"strconv"
)

func TestScale(asicdclnt *asicdServices.ASICDServicesClient, scaleCount int) {
	var count int = 0
	var maxCount int = scaleCount
	intByt2 := 1
	intByt3 := 1
	byte1 := "22"
	byte4 := "0"
	for {
		if intByt3 > 254 {
			intByt3 = 1
			intByt2++
		} else {
			intByt3++
		}
		if intByt2 > 254 {
			intByt2 = 1
		} //else {
		//intByt2++
		//}

		byte2 := strconv.Itoa(intByt2)
		byte3 := strconv.Itoa(intByt3)
		rtNet := byte1 + "." + byte2 + "." + byte3 + "." + byte4
		rv := asicdclnt.OnewayCreateIPv4Route([]*asicdInt.IPv4Route{
			&asicdInt.IPv4Route{
				rtNet,
				"255.255.255.0",
				[]*asicdInt.IPv4NextHop{
					&asicdInt.IPv4NextHop{
						NextHopIp: "11.1.10.2",
						Weight:    1,
					},
				},
			},
		})
		if rv == nil {
			count++
		} else {
			fmt.Println("Call failed", rv, "count: ", count)
			return
		}
		if maxCount == count {
			fmt.Println("Done. Total calls executed", count)
			break
		}

	}
}
