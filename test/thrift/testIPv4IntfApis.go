package testThrift

import (
	"asicdServices"
	"fmt"
)

func TestIPv4IntfApis(asicdclnt *asicdServices.ASICDServicesClient) bool {
	//Create : vlan 2, port 1 as untagged
	var vlanObj = new(asicdServices.Vlan)
	vlanObj.VlanId = int32(2)
	vlanObj.IntfList = []string{""}
	vlanObj.UntagIntfList = []string{"1"}
	_, err := asicdclnt.CreateVlan(vlanObj)
	if err != nil {
		fmt.Println("Failed to create vlan. Aborting test suite.", err)
		return false
	}
	var ipv4IntfObj = new(asicdServices.IPv4Intf)
	ipv4IntfObj.IpAddr = "20.20.20.1/24"
	ipv4IntfObj.IntfRef = "vlan2"
	_, err = asicdclnt.CreateIPv4Intf(ipv4IntfObj)
	if err != nil {
		fmt.Println("Failed to create ipv4 intf on vlan 2. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Succesfully created IPv4Intf on vlan 2")
	}
	ipv4IntfObj.IpAddr = "30.30.30.1/24"
	ipv4IntfObj.IntfRef = "fpPort3"
	_, err = asicdclnt.CreateIPv4Intf(ipv4IntfObj)
	if err != nil {
		fmt.Println("Failed to create ipv4 intf on fpPort3. Aborting test suite.", err)
		return false
	} else {
		fmt.Println("Successfully created IPv4 intf on fpPort3")
	}
	//Validate Get
	_, err = asicdclnt.GetIPv4IntfState("fpPort3")
	if err != nil {
		fmt.Println("Get call failed, when querying ipv4 intf on fpPort3. Aborting test suite.", err)
		return false
	}
	//Delete
	_, err = asicdclnt.DeleteIPv4Intf(ipv4IntfObj)
	if err != nil {
		fmt.Println("Failed to delete ipv4 intf 30.30.30.1/24 on fpPort3. Aborting test suite.", err)
		return false
	}
	ipv4IntfObj.IpAddr = "20.20.20.1/24"
	ipv4IntfObj.IntfRef = "vlan2"
	_, err = asicdclnt.DeleteIPv4Intf(ipv4IntfObj)
	if err != nil {
		fmt.Println("Failed to delete ipv4 intf 20.20.20.1/24 on vlan2. Aborting test suite.", err)
		return false
	}
	_, err = asicdclnt.DeleteVlan(vlanObj)
	if err != nil {
		fmt.Println("Failed to delete vlan. Aborting test suite.", err)
		return false
	}
	return true
}
