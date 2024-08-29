/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package resource is to manage multi-cloud infra resource
package resource

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/netutil"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	validator "github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog/log"
)

// NetworkStatus represents the status of a network resource.
type NetworkStatus string

const (

	// CRUD operations
	NetworkOnConfiguring NetworkStatus = "Configuring" // Resources are being configured.
	NetworkOnReading     NetworkStatus = "Reading"     // The network information is being read.
	NetworkOnUpdating    NetworkStatus = "Updating"    // The network is being updated.
	NetworkOnDeleting    NetworkStatus = "Deleting"    // The network is being deleted.

	// Register/deregister operations
	NetworkOnRegistering   NetworkStatus = "Registering"  // The network is being registered.
	NetworkOnDeregistering NetworkStatus = "Dergistering" // The network is being registered.

	// Available status
	NetworkAvailable NetworkStatus = "Available" // The network is fully created and ready for use.

	// In Use status
	NetworkInUse NetworkStatus = "InUse" // The network is currently in use.

	// Unknwon status
	NetworkUnknown NetworkStatus = "Unknown" // The network status is unknown.

	// Error Handling
	NetworkError              NetworkStatus = "Error"              // An error occurred during a CRUD operation.
	NetworkErrorOnConfiguring NetworkStatus = "ErrorOnConfiguring" // An error occurred during the configuring operation.
	NetworkErrorOnReading     NetworkStatus = "ErrorOnReading"     // An error occurred during the reading operation.
	NetworkErrorOnUpdating    NetworkStatus = "ErrorOnUpdating"    // An error occurred during the updating operation.
	NetworkErrorOnDeleting    NetworkStatus = "ErrorOnDeleting"    // An error occurred during the deleting operation.
	NetworkErrorOnRegistering NetworkStatus = "ErrorOnRegistering" // An error occurred during the registering operation.
)

// TbVNetReqStructLevelValidation is a function to validate 'TbVNetReq' object.
func TbVNetReqStructLevelValidation(sl validator.StructLevel) {

	u := sl.Current().Interface().(model.TbVNetReq)

	err := common.CheckString(u.Name)
	if err != nil {
		// ReportError(field interface{}, fieldName, structFieldName, tag, param string)
		sl.ReportError(u.Name, "name", "Name", err.Error(), "")
	}
}

func ValidateVNetReq(vNetReq *model.TbVNetReq) error {
	log.Debug().Msg("ValidateVNetReq")
	log.Debug().Msgf("vNetReq: %+v", vNetReq)

	err := validate.Struct(vNetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return err
		}
		return err
	}

	// Validate if vNet has at least one subnet or not
	if len(vNetReq.SubnetInfoList) == 0 {
		err := fmt.Errorf("at least one subnet is required")
		log.Error().Err(err).Msg("")
		return err
	}

	// Validate zone in each subnet
	parts := strings.SplitN(vNetReq.ConnectionName, "-", 2)
	provider := parts[0]
	region := parts[1]

	regionDetail, err := common.GetRegion(provider, region)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	zones := regionDetail.Zones
	for _, subnetInfo := range vNetReq.SubnetInfoList {
		if subnetInfo.Zone != "" {
			if !ContainsZone(zones, subnetInfo.Zone) {
				err := fmt.Errorf("invalid zone: %s", subnetInfo.Zone)
				log.Error().Err(err).Msg("")
				return err
			}
		}
	}

	// A network object for validation
	var network netutil.Network
	var subnets []netutil.Network

	network = netutil.Network{
		CidrBlock: vNetReq.CidrBlock,
	}

	for _, subnetInfo := range vNetReq.SubnetInfoList {
		subnet := netutil.Network{
			CidrBlock: subnetInfo.IPv4_CIDR,
		}
		subnets = append(subnets, subnet)
	}
	network.Subnets = subnets
	log.Debug().Msgf("network: %+v", network)

	// Validate the network object
	err = netutil.ValidateNetwork(network)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

func ContainsZone(zones []string, zone string) bool {
	for _, z := range zones {
		if z == zone {
			return true
		}
	}
	return false
}

// The spiderXxx structs are used to call the Spider REST API
// Ref:
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/api-runtime/rest-runtime/VPC-SubnetRest.go
// 2024-08-22 https://github.com/cloud-barista/cb-spider/blob/master/cloud-control-manager/cloud-driver/interfaces/resources/VPCHandler.go

// Synchronized the request body with the Spider API

// ConnectionRequest represents the request body for common use.
type spiderConnectionRequest struct {
	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
}

// VPCRegisterRequest represents the request body for registering a VPC.
type spiderVPCRegisterRequest struct {
	ConnectionName string                       `json:"ConnectionName" validate:"required" example:"aws-connection"`
	ReqInfo        spiderVPCRegisterRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderVPCRegisterRequestInfo struct {
	Name  string `json:"Name" validate:"required" example:"vpc-01"`
	CSPId string `json:"CSPId" validate:"required" example:"csp-vpc-1234"`
}

// CreateVPCRequest represents the request body for creating a VPC.
type spiderCreateVPCRequest struct {
	ConnectionName  string                     `json:"ConnectionName" validate:"required" example:"aws-connection"`
	IDTransformMode string                     `json:"IDTransformMode,omitempty" validate:"omitempty" example:"ON"` // ON: transform CSP ID, OFF: no-transform CSP ID
	ReqInfo         spiderCreateVPCRequestInfo `json:"ReqInfo" validate:"required"`
}

type spiderCreateVPCRequestInfo struct {
	Name           string                       `json:"Name" validate:"required" example:"vpc-01"`
	IPv4_CIDR      string                       `json:"IPv4_CIDR" validate:"omitempty"` // Some CSPs unsupported VPC CIDR
	SubnetInfoList []spiderAddSubnetRequestInfo `json:"SubnetInfoList" validate:"required"`
	TagList        []model.KeyValue             `json:"TagList,omitempty" validate:"omitempty"`
}

// type spiderListVPCReq struct {
// 	ConnectionName string `json:"ConnectionName" query:"ConnectionName" example:"aws-connection"`
// }

// type spiderListVPCResponse struct {
// 	Result []spiderVPCInfo `json:"vpc" validate:"required" description:"A list of VPC information"`
// }

type spiderVpcDeleteReq struct {
	ConnectionName string // Connection name for the cloud provider
}

// type spiderCspVpcDeleteReq struct {
// 	ConnectionName string // Connection name for the cloud provider
// }

type spiderBooleanInfoResp struct {
	Result string // Result of the operation
}

// type spiderGetSGOwnerVPCRequest struct {
// 	ConnectionName string `json:"ConnectionName" validate:"required" example:"aws-connection"`
// 	ReqInfo        struct {
// 		CSPId string `json:"CSPId" validate:"required" example:"csp-sg-1234"`
// 	} `json:"ReqInfo" validate:"required"`
// }

/*
	Based on polymorphism, the following Spider-related structs have been designed.
	The Spider API's requests/response bodies have been appropriately combined,
	and then `required` and `omitempty` have been appropriately assigned.
	Note - A separate struct can be created at any time
	if a conflict between `required` and `optional` is detected in a certain property.
*/

// [Note] Keep the combined structs for Spider API request bodies
// Given that API docs may not be clear about the required and optional properties currently.
// type spiderCreateVpcReq struct {
// 	spiderReqBase
// 	ReqInfo spiderVpcInfo `json:"ReqInfo" validate:"required"`
// }

// type spiderAddSubnetReq struct {
// 	spiderReqBase
// 	ReqInfo spiderSubnetInfo `json:"ReqInfo" validate:"required"`
// }

// type spiderReqBase struct {
// 	ConnectionName  string `json:"ConnectionName" validate:"required"` // Connection name for the cloud provider
// 	IDTransformMode string `json:"IDTransformMode,omitempty"`          // ID Transform mode, ON | OFF (default is ON)
// }

// [Note] Use the combined structs for Spider API response bodies
// The SpiderVpcInfo structure is a union of the properties in
// Spider's 'vpcRegisterReq', 'vpcCreateReq', and 'VPCInfo' structs.
type spiderVPCInfo struct {
	IId            model.IID          `json:"IId" validate:"required"` // {NameId, SystemId}
	IPv4_CIDR      string             `json:"IPv4_CIDR" validate:"required" example:"10.0.0.0/16" description:"The IPv4 CIDR block for the VPC"`
	SubnetInfoList []spiderSubnetInfo `json:"SubnetInfoList" validate:"required" description:"A list of subnet information associated with this VPC"`

	TagList      []model.KeyValue `json:"TagList,omitempty" validate:"omitempty" description:"A list of tags associated with this VPC"`
	KeyValueList []model.KeyValue `json:"KeyValueList,omitempty" validate:"omitempty" description:"Additional key-value pairs associated with this VPC"`
}

// CreateVNet accepts vNet creation request, creates and returns an TB vNet object
func CreateVNet(nsId string, vNetReq *model.TbVNetReq) (model.TbVNetInfo, error) {
	log.Info().Msg("CreateVNet")

	// vNet objects
	var emptyRet model.TbVNetInfo
	var vNetInfo model.TbVNetInfo

	// Set the resource type
	resourceType := model.StrVNet
	childResourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = validate.Struct(vNetReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object
	uuid := common.GenUid()
	vNetInfo.Name = vNetReq.Name
	vNetInfo.Id = vNetReq.Name
	vNetInfo.Uuid = uuid
	vNetInfo.ConnectionName = vNetReq.ConnectionName
	vNetInfo.Description = vNetReq.Description
	vNetInfo.TagList = vNetReq.TagList

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetInfo.Id)
	// Check if the vNet already exists or not
	exists, err := CheckResource(nsId, resourceType, vNetInfo.Id)
	if exists {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("already exists, vNet: %s", vNetInfo.Id)
		return emptyRet, err
	}
	if err != nil {
		log.Error().Err(err).Msg("")
		err := fmt.Errorf("failed to check if the vNet (%s) exists or not", vNetInfo.Id)
		return emptyRet, err
	}

	// Note: Set subnetInfoList in vNetInfo in advance
	//       since each subnet UUID must be consistent
	for _, subnetInfo := range vNetReq.SubnetInfoList {
		vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList, model.TbSubnetInfo{
			Id:        subnetInfo.Name,
			Name:      subnetInfo.Name,
			Uuid:      common.GenUid(),
			IPv4_CIDR: subnetInfo.IPv4_CIDR,
			Zone:      subnetInfo.Zone,
			TagList:   subnetInfo.TagList,
		})
	}

	log.Debug().Msgf("vNetInfo: %+v", vNetInfo)

	// [Set status]
	vNetInfo.Status = string(NetworkOnConfiguring)

	// Save the current operation status and the vNet object
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set request body to create a vNet and subnets
	spReqt := spiderCreateVPCRequest{}
	spReqt.ConnectionName = vNetReq.ConnectionName
	spReqt.ReqInfo.Name = uuid
	spReqt.ReqInfo.IPv4_CIDR = vNetReq.CidrBlock

	// Note: Use the subnets in the vNetInfo object (instead of the vNetReq object)
	//       since each subnet UUID must be consistent
	for _, subnetInfo := range vNetInfo.SubnetInfoList {
		spReqt.ReqInfo.SubnetInfoList = append(spReqt.ReqInfo.SubnetInfoList, spiderAddSubnetRequestInfo{
			Name:      subnetInfo.Uuid,
			IPv4_CIDR: subnetInfo.IPv4_CIDR,
			Zone:      subnetInfo.Zone,
			TagList:   subnetInfo.TagList,
		})
	}

	log.Debug().Msgf("spReqt: %+v", spReqt)

	client := resty.New()
	method := "POST"
	var spResp spiderVPCInfo

	// API to create a vNet
	url := fmt.Sprintf("%s/vpc", model.SpiderRestUrl)

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspVNetId = spResp.IId.SystemId
	vNetInfo.CspVNetName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList
	vNetInfo.TagList = spResp.TagList

	// Note: Check one by one and update the vNet object with the response from the Spider
	//       since the order may differ different between slices
	for _, spSubnetInfo := range spResp.SubnetInfoList {
		for i, tbSubnetInfo := range vNetInfo.SubnetInfoList {
			if tbSubnetInfo.Uuid == spSubnetInfo.IId.NameId {
				vNetInfo.SubnetInfoList[i].ConnectionName = vNetInfo.ConnectionName
				vNetInfo.SubnetInfoList[i].CspVNetId = spResp.IId.SystemId
				vNetInfo.SubnetInfoList[i].CspVNetName = spResp.IId.NameId
				vNetInfo.SubnetInfoList[i].Status = string(NetworkAvailable)
				vNetInfo.SubnetInfoList[i].CspSubnetId = spSubnetInfo.IId.SystemId
				vNetInfo.SubnetInfoList[i].CspSubnetName = spSubnetInfo.IId.NameId
				vNetInfo.SubnetInfoList[i].KeyValueList = spSubnetInfo.KeyValueList
				vNetInfo.SubnetInfoList[i].TagList = spSubnetInfo.TagList
				vNetInfo.SubnetInfoList[i].Zone = spSubnetInfo.Zone
				vNetInfo.SubnetInfoList[i].IPv4_CIDR = spSubnetInfo.IPv4_CIDR
			}
		}
	}

	// [Set status]
	if len(vNetInfo.SubnetInfoList) == 0 {
		vNetInfo.Status = string(NetworkAvailable)
	} else if len(vNetInfo.SubnetInfoList) > 0 {
		vNetInfo.Status = string(NetworkInUse)
	} else {
		vNetInfo.Status = string(NetworkUnknown)
		log.Warn().Msgf("The status of the vNet (%s) is unknown", vNetInfo.Id)
	}

	log.Debug().Msgf("vNetInfo: %+v", vNetInfo)

	// Save vNet object into the key-value store
	value, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = kvstore.Put(vNetKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Save subnet objects into the key-value store
	for _, subnetInfo := range vNetInfo.SubnetInfoList {
		// Set a subnetKey for the subnet object
		subnetKey := common.GenChildResourceKey(nsId, childResourceType, vNetInfo.Id, subnetInfo.Id)
		value, err := json.Marshal(subnetInfo)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}

		err = kvstore.Put(subnetKey, string(value))
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Check if the vNet info is stored
	vNetKv, err := kvstore.GetKv(vNetKey)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetInfo.Id)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return vNetInfo, nil
}

func GetVNet(nsId string, vNetId string) (model.TbVNetInfo, error) {
	log.Info().Msg("GetVNet")

	// vNet object
	var emptyRet model.TbVNetInfo
	var vNetInfo model.TbVNetInfo

	// Set the resource type
	resourceType := model.StrVNet
	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)

	// Check the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Read the stored vNet info
	keyValue, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if keyValue == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Get a vNet and subnets
	client := resty.New()
	method := "GET"
	spReqt := common.NoBody
	var spResp spiderVPCInfo

	// API to create a vNet
	url := fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, vNetInfo.CspVNetName)
	queryParams := "?ConnectionName=" + vNetInfo.ConnectionName
	url += queryParams

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspVNetId = spResp.IId.SystemId
	vNetInfo.CspVNetName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList
	vNetInfo.TagList = spResp.TagList

	// Save the current operation status and the vNet object
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	return vNetInfo, nil
}

// DeleteVNet accepts vNet creation request, creates and returns an TB vNet object
func DeleteVNet(nsId string, vNetId string, withSubnets string) (model.SimpleMsg, error) {
	log.Info().Msg("DeleteVNet")

	// vNet object
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	// Set the resource type
	resourceType := model.StrVNet

	// Check the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Validate options: withSubnets
	if withSubnets != "" && withSubnets != "true" && withSubnets != "false" {
		errMsg := fmt.Errorf("invalid option, withSubnets (%s)", withSubnets)
		log.Warn().Msgf(errMsg.Error())
		return emptyRet, errMsg
	}
	if withSubnets == "" {
		withSubnets = "false"
	}

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)
	// Read the stored subnets
	subnetsKv, err := kvstore.GetKvList(vNetKey + "/subnet")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("subnetsKv: %+v", subnetsKv)

	if withSubnets == "false" && len(subnetsKv) > 0 {
		err := fmt.Errorf("the vNet (%s) is in-use, may have subnets", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Delete the subnets associated with the vNet
	for _, kv := range subnetsKv {
		subnet := model.TbSubnetInfo{}
		err = json.Unmarshal([]byte(kv.Value), &subnet)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		_, err := DeleteSubnet(nsId, vNetId, subnet.Id)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Read the stored vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// vNet object
	var vNetInfo model.TbVNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set status to 'Deleting'
	vNetInfo.Status = string(NetworkOnDeleting)
	// Save the status
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Delete the vNet
	spReqt := spiderVpcDeleteReq{}
	spReqt.ConnectionName = vNetInfo.ConnectionName

	// API to delete a vNet
	url := fmt.Sprintf("%s/vpc/%s", model.SpiderRestUrl, vNetInfo.CspVNetName)

	var spResp spiderBooleanInfoResp

	client := resty.New()
	method := "DELETE"

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	ok, err := strconv.ParseBool(spResp.Result)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !ok {
		err := fmt.Errorf("failed to delete the vNet (%s)", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Delete the saved the vNet info
	err = kvstore.Delete(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the vNet (%s) has been deleted", vNetId)

	return ret, nil
}

// RegisterVNet accepts vNet registration request, register and returns an TB vNet object
func RegisterVNet(nsId string, vNetRegisterReq *model.TbRegisterVNetReq) (model.TbVNetInfo, error) {
	log.Info().Msg("RegisterVNet")

	// vNet objects
	var emptyRet model.TbVNetInfo
	var vNetInfo model.TbVNetInfo

	// Set the resource type
	resourceType := model.StrVNet
	childResourceType := model.StrSubnet

	// Validate the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = validate.Struct(vNetRegisterReq)
	if err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object
	uuid := common.GenUid()
	vNetInfo.Id = vNetRegisterReq.Name
	vNetInfo.Name = vNetRegisterReq.Name
	vNetInfo.Uuid = uuid
	vNetInfo.ConnectionName = vNetRegisterReq.ConnectionName
	vNetInfo.Description = vNetRegisterReq.Description

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetRegisterReq.Name)
	// Check if the vNet already exists or not
	exists, err := CheckResource(nsId, resourceType, vNetRegisterReq.Name)
	if exists {
		err := fmt.Errorf("already exists, vNet: %s", vNetRegisterReq.Name)
		return emptyRet, err
	}
	if err != nil {
		err := fmt.Errorf("failed to check if the vNet (%s) exists or not", vNetRegisterReq.Name)
		return emptyRet, err
	}

	// [Set status]
	vNetInfo.Status = string(NetworkOnRegistering)

	// Save the current operation status and the vNet object
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		return emptyRet, err
	}

	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		return emptyRet, err
	}

	// Register a vNet that has already been created externally
	spReqt := spiderVPCRegisterRequest{}
	spReqt.ConnectionName = vNetRegisterReq.ConnectionName
	spReqt.ReqInfo.Name = common.GenUid()
	spReqt.ReqInfo.CSPId = vNetRegisterReq.CspVNetId

	client := resty.New()
	method := "POST"
	var spResp spiderVPCInfo

	// API to register a vNet
	url := fmt.Sprintf("%s/regvpc", model.SpiderRestUrl)

	// if option == "register" && vNetReq.CspVNetId == "" {
	// 	url = fmt.Sprintf("%s/vpc/%s", common.SpiderRestUrl, vNetReq.Name)
	// 	method = "GET"
	// } else if option == "register" && vNetReq.CspVNetId != "" {
	// 	url = fmt.Sprintf("%s/regvpc", common.SpiderRestUrl)
	// }

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set the vNet object with the response from the Spider
	vNetInfo.CspVNetId = spResp.IId.SystemId
	vNetInfo.CspVNetName = spResp.IId.NameId
	vNetInfo.CidrBlock = spResp.IPv4_CIDR
	vNetInfo.KeyValueList = spResp.KeyValueList
	vNetInfo.TagList = spResp.TagList

	if vNetRegisterReq.CspVNetId == "" {
		vNetInfo.SystemLabel = "Registered from CB-Spider resource"
	} else if vNetRegisterReq.CspVNetId != "" {
		vNetInfo.SystemLabel = "Registered from CSP resource"
	}

	// Note: Check one by one and update the vNet object with the response from the Spider
	//       since the order may differ different between slices
	for i, spSubnetInfo := range spResp.SubnetInfoList {
		subnet := model.TbSubnetInfo{
			Id:             fmt.Sprintf("reg-subnet-%02d", i+1),
			Name:           fmt.Sprintf("reg-subnet-%02d", i+1),
			ConnectionName: vNetInfo.ConnectionName,
			CspVNetId:      spResp.IId.SystemId,
			CspVNetName:    spResp.IId.NameId,
			Status:         string(NetworkUnknown),
			CspSubnetId:    spSubnetInfo.IId.SystemId,
			CspSubnetName:  spSubnetInfo.IId.NameId,
			KeyValueList:   spSubnetInfo.KeyValueList,
			TagList:        spSubnetInfo.TagList,
			Zone:           spSubnetInfo.Zone,
			IPv4_CIDR:      spSubnetInfo.IPv4_CIDR,
		}
		vNetInfo.SubnetInfoList = append(vNetInfo.SubnetInfoList, subnet)

		// Set a subnetKey for the subnet object
		subnetKey := common.GenChildResourceKey(nsId, childResourceType, vNetInfo.Id, subnet.Id)
		// Save the subnet object
		value, err := json.Marshal(subnet)
		if err != nil {
			return emptyRet, err
		}
		err = kvstore.Put(subnetKey, string(value))
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	log.Debug().Msgf("vNetInfo: %+v", vNetInfo)

	// [Set status]
	vNetInfo.Status = string(NetworkAvailable)

	// Put vNet object into the key-value store
	value, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(value))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Check if the vNet info is stored
	keyValue, err := kvstore.GetKv(vNetKey)

	if keyValue == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetRegisterReq.Name)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	err = json.Unmarshal([]byte(keyValue.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
	}
	return vNetInfo, nil
}

// DeregisterVNet accepts vNet unregistration request, deregister and returns the result
func DeregisterVNet(nsId string, vNetId string, withSubnets string) (model.SimpleMsg, error) {
	log.Info().Msg("DeregisterVNet")

	// vNet object
	var emptyRet model.SimpleMsg
	var ret model.SimpleMsg

	// Set the resource type
	resourceType := model.StrVNet

	// Check the input parameters
	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = common.CheckString(vNetId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Validate options: withSubnets
	if withSubnets != "" && withSubnets != "true" && withSubnets != "false" {
		errMsg := fmt.Errorf("invalid option, withSubnets (%s)", withSubnets)
		log.Warn().Msgf(errMsg.Error())
		return emptyRet, errMsg
	}
	if withSubnets == "" {
		withSubnets = "false"
	}

	// Set a vNetKey for the vNet object
	vNetKey := common.GenResourceKey(nsId, resourceType, vNetId)
	// Read the stored subnets
	subnetsKv, err := kvstore.GetKvList(vNetKey + "/subnet")
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	log.Debug().Msgf("subnetsKv: %+v", subnetsKv)

	if withSubnets == "false" && len(subnetsKv) > 0 {
		err := fmt.Errorf("the vNet (%s) is in-use, may have subnets", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Delete the subnets associated with the vNet
	for _, kv := range subnetsKv {
		subnet := model.TbSubnetInfo{}
		err = json.Unmarshal([]byte(kv.Value), &subnet)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
		_, err := DeleteSubnet(nsId, vNetId, subnet.Id)
		if err != nil {
			log.Error().Err(err).Msg("")
			return emptyRet, err
		}
	}

	// Read the stored vNet info
	vNetKv, err := kvstore.GetKv(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if vNetKv == (kvstore.KeyValue{}) {
		err := fmt.Errorf("does not exist, vNet: %s", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// vNet object
	var vNetInfo model.TbVNetInfo
	err = json.Unmarshal([]byte(vNetKv.Value), &vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Set status to 'Deleting'
	vNetInfo.Status = string(NetworkOnDeleting)
	// Save the status
	val, err := json.Marshal(vNetInfo)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	err = kvstore.Put(vNetKey, string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Via Spider] Deregister the vNet
	spReqt := spiderConnectionRequest{}
	spReqt.ConnectionName = vNetInfo.ConnectionName

	// API to delete a vNet
	url := fmt.Sprintf("%s/regvpc/%s", model.SpiderRestUrl, vNetInfo.CspVNetName)

	var spResp spiderBooleanInfoResp

	client := resty.New()
	method := "DELETE"

	err = common.ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		common.SetUseBody(spReqt),
		&spReqt,
		&spResp,
		common.MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	ok, err := strconv.ParseBool(spResp.Result)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}
	if !ok {
		err := fmt.Errorf("failed to deregister the vNet (%s)", vNetId)
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// Delete the saved the vNet info
	err = kvstore.Delete(vNetKey)
	if err != nil {
		log.Error().Err(err).Msg("")
		return emptyRet, err
	}

	// [Output] the message
	ret.Message = fmt.Sprintf("the vnet (%s) has been deregistered", vNetId)

	return ret, nil
}
