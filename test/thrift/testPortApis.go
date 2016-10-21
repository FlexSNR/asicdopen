package testThrift

import (
	"asicdServices"
	"fmt"
)

func TestPortApis(asicdclnt *asicdServices.ASICDServicesClient) bool {
	//GetBulk config
	bulkCfg, err := asicdclnt.GetBulkPort(0, 255)
	if err != nil {
		fmt.Println("Failed to get bulk port config. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Get bulk port config successful !")
	}
	//GetBulk state
	_, err = asicdclnt.GetBulkPortState(0, 255)
	if err != nil {
		fmt.Println("Failed to get bulk port state. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Get bulk port state successful !")
	}
	//Update port config
	var pCfg *asicdServices.Port = new(asicdServices.Port)
	*pCfg = *bulkCfg.PortList[0]
	pCfg.AdminState = "UP"
	pCfg.Speed = 1000
	_, err = asicdclnt.UpdatePort(bulkCfg.PortList[0], pCfg, []bool{false, false, false, false, true, false, true, false, false, false, false, false}, nil)
	if err != nil {
		fmt.Println("Failed to update port config. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Successfully updated port config for fpPort1")
	}
	//Verify update
	bulkCfg, err = asicdclnt.GetBulkPort(0, 10)
	if err != nil {
		fmt.Println("Failed to get bulk port config. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("fpPort1 after applying update - ", bulkCfg.PortList[0])
	}
	return true
}
