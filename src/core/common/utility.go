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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"math/rand"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
	uid "github.com/rs/xid"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v2"

	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// MCI utilities

// GenUid is func to return a uid string
func GenUid() string {
	return uid.New().String()
}

// GenRandomPassword is func to return a RandomPassword
func GenRandomPassword(length int) string {
	rand.Seed(time.Now().Unix())

	charset := "A1!$"
	shuff := []rune(charset)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})
	randomString := GenUid()
	if len(randomString) < length {
		randomString = randomString + GenUid()
	}
	reducedString := randomString[0 : length-len(charset)]
	reducedString = reducedString + string(shuff)

	shuff = []rune(reducedString)
	rand.Shuffle(len(shuff), func(i, j int) {
		shuff[i], shuff[j] = shuff[j], shuff[i]
	})

	pw := string(shuff)

	return pw
}

// RandomSleep is func to make a caller waits for during random time seconds (random value within x~y)
func RandomSleep(from int, to int) {
	if from > to {
		tmp := from
		from = to
		to = tmp
	}
	t := to - from
	rand.Seed(time.Now().UnixNano())
	n := rand.Intn(t * 1000)
	time.Sleep(time.Duration(n) * time.Millisecond)
}

// GetFuncName is func to get the name of the running function
func GetFuncName() string {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])
	return f.Name()
}

// CheckString is func to check string by the given rule `[a-z]([-a-z0-9]*[a-z0-9])?`
func CheckString(name string) error {

	if name == "" {
		err := fmt.Errorf("The provided string is empty")
		return err
	}

	r, _ := regexp.Compile("(?i)[a-z]([-a-z0-9+]*[a-z0-9])?")
	filtered := r.FindString(name)

	if filtered != name {
		err := fmt.Errorf(name + ": The name must follow these rules: " +
			"1. The first character must be a letter (case-insensitive). " +
			"2. All following characters can be a dash, letter (case-insensitive), digit, or +. " +
			"3. The last character cannot be a dash.")
		return err
	}

	return nil
}

// ToLower is func to change strings (_ to -, " " to -, to lower string ) (deprecated soon)
func ToLower(name string) string {
	out := strings.ReplaceAll(name, "_", "-")
	out = strings.ReplaceAll(out, " ", "-")
	out = strings.ToLower(out)
	return out
}

// ChangeIdString is func to change strings in id or name (special chars to -, to lower string )
func ChangeIdString(name string) string {
	// Regex for letters and numbers
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	changedString := strings.ToLower(reg.ReplaceAllString(name, "-"))
	if changedString[len(changedString)-1:] == "-" {
		changedString += "r"
	}
	return changedString
}

// GenMciKey is func to generate a key used in keyValue store
func GenMciKey(nsId string, mciId string, vmId string) string {

	if vmId != "" {
		return "/ns/" + nsId + "/mci/" + mciId + "/vm/" + vmId
	} else if mciId != "" {
		return "/ns/" + nsId + "/mci/" + mciId
	} else if nsId != "" {
		return "/ns/" + nsId
	} else {
		return ""
	}

}

// GenMciSubGroupKey is func to generate a key from subGroupId used in keyValue store
func GenMciSubGroupKey(nsId string, mciId string, groupId string) string {

	return "/ns/" + nsId + "/mci/" + mciId + "/subgroup/" + groupId

}

// GenMciPolicyKey is func to generate Mci policy key
func GenMciPolicyKey(nsId string, mciId string, vmId string) string {
	if vmId != "" {
		return "/ns/" + nsId + "/policy/mci/" + mciId + "/vm/" + vmId
	} else if mciId != "" {
		return "/ns/" + nsId + "/policy/mci/" + mciId
	} else if nsId != "" {
		return "/ns/" + nsId
	} else {
		return ""
	}
}

// GenConnectionKey is func to generate a key for connection info
func GenConnectionKey(connectionId string) string {
	return "/connection/" + connectionId
}

// GenCredentialHolderKey is func to generate a key for credentialHolder info
func GenCredentialHolderKey(holderId string) string {
	return "/credentialHolder/" + holderId
}

// LookupKeyValueList is func to lookup model.KeyValue list
func LookupKeyValueList(kvl []model.KeyValue, key string) string {
	for _, v := range kvl {
		if v.Key == key {
			return v.Value
		}
	}
	return ""
}

// PrintJsonPretty is func to print JSON pretty with indent
func PrintJsonPretty(v interface{}) {
	prettyJSON, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%+v\n", v)
	} else {
		fmt.Printf("%s\n", string(prettyJSON))
	}
}

// GenResourceKey is func to generate a key from resource type and id
func GenResourceKey(nsId string, resourceType string, resourceId string) string {

	if resourceType == model.StrImage ||
		resourceType == model.StrCustomImage ||
		resourceType == model.StrSSHKey ||
		resourceType == model.StrSpec ||
		resourceType == model.StrVNet ||
		resourceType == model.StrSecurityGroup ||
		resourceType == model.StrDataDisk {
		//resourceType == "publicIp" ||
		//resourceType == "vNic" {
		return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
	} else {
		return "/invalidKey"
	}
}

// GenChildResourceKey is func to generate a key from resource type and id
func GenChildResourceKey(nsId string, resourceType string, parentResourceId string, resourceId string) string {

	if resourceType == model.StrSubnet {
		parentResourceType := model.StrVNet
		// return "/ns/" + nsId + "/resources/" + resourceType + "/" + resourceId
		return fmt.Sprintf("/ns/%s/resources/%s/%s/%s/%s", nsId, parentResourceType, parentResourceId, resourceType, resourceId)
	} else {
		return "/invalidKey"
	}
}

// GetConnConfig is func to get connection config
func GetConnConfig(ConnConfigName string) (model.ConnConfig, error) {

	connConfig := model.ConnConfig{}

	key := GenConnectionKey(ConnConfigName)
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ConnConfig{}, err
	}
	if keyValue == (kvstore.KeyValue{}) {
		return model.ConnConfig{}, fmt.Errorf("Cannot find the model.ConnConfig " + key)
	}
	err = json.Unmarshal([]byte(keyValue.Value), &connConfig)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ConnConfig{}, err
	}

	return connConfig, nil
}

// CheckConnConfigAvailable is func to check if connection config is available by checking allkeypair list
func CheckConnConfigAvailable(connConfigName string) (bool, error) {

	var callResult interface{}
	client := resty.New()
	url := model.SpiderRestUrl + "/allkeypair"
	method := "GET"
	requestBody := model.SpiderConnectionName{}
	requestBody.ConnectionName = connConfigName

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		ShortDuration,
	)

	if err != nil {
		//log.Info().Err(err).Msg("")
		return false, err
	}

	return true, nil
}

// CheckSpiderStatus is func to check if CB-Spider is ready
func CheckSpiderReady() error {

	var callResult interface{}
	client := resty.New()
	url := model.SpiderRestUrl + "/readyz"
	method := "GET"
	requestBody := NoBody

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		VeryShortDuration,
	)

	if err != nil {
		//log.Err(err).Msg("")
		return err
	}

	return nil
}

// GetConnConfigList is func to list filtered connection configs
func GetConnConfigList(filterCredentialHolder string, filterVerified bool, filterRegionRepresentative bool) (model.ConnConfigList, error) {
	var filteredConnections model.ConnConfigList
	var tmpConnections model.ConnConfigList

	key := "/connection"
	keyValue, err := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ConnConfigList{}, err
	}
	if keyValue != nil {
		for _, v := range keyValue {
			tempObj := model.ConnConfig{}
			err = json.Unmarshal([]byte(v.Value), &tempObj)
			if err != nil {
				log.Error().Err(err).Msg("")
				return filteredConnections, err
			}
			filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, tempObj)
		}
	} else {
		return model.ConnConfigList{}, nil
	}

	// filter by credential holder
	if filterCredentialHolder != "" {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if strings.EqualFold(connConfig.CredentialHolder, filterCredentialHolder) {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = model.ConnConfigList{}
	}

	// filter only verified
	if filterVerified {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if connConfig.Verified {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = model.ConnConfigList{}
	}

	// filter only region representative
	if filterRegionRepresentative {
		for _, connConfig := range filteredConnections.Connectionconfig {
			if connConfig.RegionRepresentative {
				tmpConnections.Connectionconfig = append(tmpConnections.Connectionconfig, connConfig)
			}
		}
		filteredConnections = tmpConnections
		tmpConnections = model.ConnConfigList{}
	}
	//log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
	return filteredConnections, nil
}

// RegisterAllCloudInfo is func to register all cloud info from asset to CB-Spider
func RegisterAllCloudInfo() error {
	for providerName := range RuntimeCloudInfo.CSPs {
		err := RegisterCloudInfo(providerName)
		if err != nil {
			log.Error().Err(err).Msg("")
		}
	}
	return nil
}

// GetProviderList is func to list all cloud providers
func GetProviderList() (*model.IdList, error) {
	providers := model.IdList{}
	for providerName := range RuntimeCloudInfo.CSPs {
		providers.IdList = append(providers.IdList, providerName)
	}
	return &providers, nil
}

// RegisterCloudInfo is func to register cloud info from asset to CB-Spider
func RegisterCloudInfo(providerName string) error {

	driverName := RuntimeCloudInfo.CSPs[providerName].Driver

	client := resty.New()
	url := model.SpiderRestUrl + "/driver"
	method := "POST"
	var callResult model.CloudDriverInfo
	requestBody := model.CloudDriverInfo{ProviderName: strings.ToUpper(providerName), DriverName: driverName, DriverLibFileName: driverName}

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	for regionName, _ := range RuntimeCloudInfo.CSPs[providerName].Regions {
		err := RegisterRegionZone(providerName, regionName)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	}

	return nil
}

// RegisterRegionZone is func to register all regions to CB-Spider
func RegisterRegionZone(providerName string, regionName string) error {
	client := resty.New()
	url := model.SpiderRestUrl + "/region"
	method := "POST"
	var callResult model.SpiderRegionZoneInfo
	requestBody := model.SpiderRegionZoneInfo{ProviderName: strings.ToUpper(providerName), RegionName: regionName}

	// register representative regionZone (region only)
	requestBody.RegionName = providerName + "-" + regionName
	keyValueInfoList := []model.KeyValue{}

	if len(RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones) > 0 {
		keyValueInfoList = []model.KeyValue{
			{Key: "Region", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].RegionId},
			{Key: "Zone", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones[0]},
		}
	} else {
		keyValueInfoList = []model.KeyValue{
			{Key: "Region", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].RegionId},
			{Key: "Zone", Value: "N/A"},
		}
	}
	requestBody.KeyValueInfoList = keyValueInfoList

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	// register all regionZones
	for _, zoneName := range RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones {
		requestBody.RegionName = providerName + "-" + regionName + "-" + zoneName
		keyValueInfoList := []model.KeyValue{
			{Key: "Region", Value: RuntimeCloudInfo.CSPs[providerName].Regions[regionName].RegionId},
			{Key: "Zone", Value: zoneName},
		}
		requestBody.AvailableZoneList = RuntimeCloudInfo.CSPs[providerName].Regions[regionName].Zones
		requestBody.KeyValueInfoList = keyValueInfoList

		err := ExecuteHttpRequest(
			client,
			method,
			url,
			nil,
			SetUseBody(requestBody),
			&requestBody,
			&callResult,
			MediumDuration,
		)

		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}

	}

	return nil
}

var privateKeyStore = make(map[string]*rsa.PrivateKey)
var mu sync.Mutex // Concurrency safety

// GetPublicKeyForCredentialEncryption generates an RSA key pair,
// stores the private key in memory, and returns the public key along with its token ID.
func GetPublicKeyForCredentialEncryption() (model.PublicKeyResponse, error) {

	privateKey, err := rsa.GenerateKey(crand.Reader, 4096)
	if err != nil {
		return model.PublicKeyResponse{}, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	uid := GenUid()

	mu.Lock()
	privateKeyStore[uid] = privateKey
	mu.Unlock()

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey),
	})

	return model.PublicKeyResponse{
		PublicKeyTokenId: uid,
		PublicKey:        string(publicKeyPEM),
	}, nil
}

// hashFunction is the hash function used for RSA-OAEP decryption
var hashFunction = sha256.New

// unpad function to remove padding after AES decryption
func unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	unpadding := int(data[length-1])
	if unpadding > blockSize || unpadding > length {
		return nil, fmt.Errorf("invalid padding size")
	}
	return data[:(length - unpadding)], nil
}

// RegisterCredential is func to register credential and all related connection configs
func RegisterCredential(req model.CredentialReq) (model.CredentialInfo, error) {

	mu.Lock()
	privateKey, exists := privateKeyStore[req.PublicKeyTokenId]
	mu.Unlock()

	if !exists {
		return model.CredentialInfo{}, fmt.Errorf("private key not found for token ID: %s", req.PublicKeyTokenId)
	}

	// PrintJsonPretty(req)

	// Decrypt the AES key
	encryptedAesKey, err := base64.StdEncoding.DecodeString(req.EncryptedClientAesKeyByPublicKey)
	if err != nil {
		return model.CredentialInfo{}, fmt.Errorf("failed to decode encrypted AES key: %w", err)
	}

	aesKey, err := rsa.DecryptOAEP(
		sha256.New(), crand.Reader, privateKey, encryptedAesKey, nil,
	)
	if err != nil {
		return model.CredentialInfo{}, fmt.Errorf("failed to decrypt AES key: %w", err)
	}

	// Clear AES key from memory after use
	defer func() {
		for i := range aesKey {
			aesKey[i] = 0
		}
	}()

	decryptedKeyValueList := make([]model.KeyValue, len(req.CredentialKeyValueList))

	// Decrypt all encrypted values and populate the new list
	for i, keyValue := range req.CredentialKeyValueList {
		encryptedBytes, err := base64.StdEncoding.DecodeString(keyValue.Value)
		if err != nil {
			log.Error().Err(err).Msg("")
			return model.CredentialInfo{}, fmt.Errorf("failed to decode encrypted value: %w", err)
		}

		aesCipher, err := aes.NewCipher(aesKey)
		if err != nil {
			return model.CredentialInfo{}, fmt.Errorf("failed to create AES cipher: %w", err)
		}

		iv := encryptedBytes[:aes.BlockSize]
		ciphertext := encryptedBytes[aes.BlockSize:]
		aesBlock := cipher.NewCBCDecrypter(aesCipher, iv)
		decryptedValue := make([]byte, len(ciphertext))
		aesBlock.CryptBlocks(decryptedValue, ciphertext)

		// Remove padding
		decryptedValue, err = unpad(decryptedValue, aes.BlockSize)
		if err != nil {
			return model.CredentialInfo{}, fmt.Errorf("failed to unpad decrypted value: %w", err)
		}

		decryptedKeyValueList[i] = model.KeyValue{
			Key:   keyValue.Key,
			Value: string(decryptedValue),
		}
	}

	// Delete the private key from memory after use
	mu.Lock()
	delete(privateKeyStore, req.PublicKeyTokenId)
	mu.Unlock()

	req.CredentialHolder = strings.ToLower(req.CredentialHolder)
	req.ProviderName = strings.ToLower(req.ProviderName)
	genneratedCredentialName := req.CredentialHolder + "-" + req.ProviderName
	if req.CredentialHolder == model.DefaultCredentialHolder {
		// credential with default credential holder (e.g., admin) has no prefix
		genneratedCredentialName = req.ProviderName
	}

	// replace `\\n` with `\n` in the value to restore the original PEM value
	for i, keyValue := range decryptedKeyValueList {
		decryptedKeyValueList[i].Value = strings.ReplaceAll(keyValue.Value, "\\n", "\n")
	}

	reqToSpider := model.CredentialInfo{
		CredentialName:   genneratedCredentialName,
		ProviderName:     strings.ToUpper(req.ProviderName),
		KeyValueInfoList: decryptedKeyValueList,
	}

	client := resty.New()
	url := model.SpiderRestUrl + "/credential"
	method := "POST"
	var callResult model.CredentialInfo
	requestBody := reqToSpider

	//PrintJsonPretty(requestBody)

	err = ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.CredentialInfo{}, err
	}
	//PrintJsonPretty(callResult)

	callResult.CredentialHolder = req.CredentialHolder
	callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	for callResultKey := range callResult.KeyValueInfoList {
		callResult.KeyValueInfoList[callResultKey].Value = "************"
	}

	// TODO: add code to register CredentialHolder object

	cloudInfo, err := GetCloudInfo()
	if err != nil {
		return callResult, err
	}
	cspDetail, ok := cloudInfo.CSPs[callResult.ProviderName]
	if !ok {
		return callResult, fmt.Errorf("cloudType '%s' not found", callResult.ProviderName)
	}

	// register connection config for all regions with the credential
	allRegisteredRegions, err := GetRegionList()
	if err != nil {
		return callResult, err
	}
	for _, region := range allRegisteredRegions.Region {
		if strings.ToLower(region.ProviderName) == callResult.ProviderName {
			configName := callResult.CredentialHolder + "-" + region.RegionName
			if callResult.CredentialHolder == model.DefaultCredentialHolder {
				configName = region.RegionName
			}
			connConfig := model.ConnConfig{
				ConfigName:         configName,
				ProviderName:       strings.ToUpper(callResult.ProviderName),
				DriverName:         cspDetail.Driver,
				CredentialName:     callResult.CredentialName,
				RegionZoneInfoName: region.RegionName,
				CredentialHolder:   req.CredentialHolder,
			}
			_, err := RegisterConnectionConfig(connConfig)
			if err != nil {
				log.Error().Err(err).Msg("")
				return callResult, err
			}
		}
	}

	validate := true
	// filter only verified
	if validate {
		allConnections, err := GetConnConfigList(req.CredentialHolder, false, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}

		filteredConnections := model.ConnConfigList{}
		for _, connConfig := range allConnections.Connectionconfig {
			if strings.EqualFold(callResult.ProviderName, connConfig.ProviderName) {
				connConfig.ProviderName = strings.ToLower(connConfig.ProviderName)
				filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, connConfig)
			}
		}

		var wg sync.WaitGroup
		results := make(chan model.ConnConfig, len(filteredConnections.Connectionconfig))

		for _, connConfig := range filteredConnections.Connectionconfig {
			wg.Add(1)
			go func(connConfig model.ConnConfig) {
				defer wg.Done()
				RandomSleep(0, 30)
				verified, err := CheckConnConfigAvailable(connConfig.ConfigName)
				if err != nil {
					log.Error().Err(err).Msgf("Cannot check model.ConnConfig %s is available", connConfig.ConfigName)
				}
				connConfig.Verified = verified
				if verified {
					regionInfo, err := GetRegion(connConfig.ProviderName, connConfig.RegionDetail.RegionName)
					if err != nil {
						log.Error().Err(err).Msgf("Cannot get region for %s", connConfig.RegionDetail.RegionName)
						connConfig.Verified = false
					} else {
						connConfig.RegionDetail = regionInfo
					}
				}
				results <- connConfig
			}(connConfig)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for result := range results {
			if result.Verified {
				key := GenConnectionKey(result.ConfigName)
				val, err := json.Marshal(result)
				if err != nil {
					return model.CredentialInfo{}, err
				}
				err = kvstore.Put(string(key), string(val))
				if err != nil {
					return callResult, err
				}
			}
		}
	}

	setRegionRepresentative := true
	if setRegionRepresentative {
		allConnections, err := GetConnConfigList(req.CredentialHolder, false, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}

		filteredConnections := model.ConnConfigList{}
		for _, connConfig := range allConnections.Connectionconfig {
			if strings.EqualFold(req.ProviderName, connConfig.ProviderName) {
				filteredConnections.Connectionconfig = append(filteredConnections.Connectionconfig, connConfig)
			}
		}
		log.Info().Msgf("Filtered connection config count: %d", len(filteredConnections.Connectionconfig))
		regionRepresentative := make(map[string]model.ConnConfig)
		for _, connConfig := range allConnections.Connectionconfig {
			prefix := req.ProviderName + "-" + connConfig.RegionDetail.RegionName
			if strings.EqualFold(connConfig.RegionZoneInfoName, prefix) {
				if _, exists := regionRepresentative[prefix]; !exists {
					regionRepresentative[prefix] = connConfig
				}
			}
		}
		for _, connConfig := range regionRepresentative {
			connConfig.RegionRepresentative = true
			key := GenConnectionKey(connConfig.ConfigName)
			val, err := json.Marshal(connConfig)
			if err != nil {
				return callResult, err
			}
			err = kvstore.Put(string(key), string(val))
			if err != nil {
				return callResult, err
			}
		}
	}

	verifyRegionRepresentativeAndUpdateZone := true
	if verifyRegionRepresentativeAndUpdateZone {
		verifiedConnections, err := GetConnConfigList(req.CredentialHolder, true, false)
		if err != nil {
			log.Error().Err(err).Msg("")
			return callResult, err
		}
		allRepresentativeRegionConnections, err := GetConnConfigList(req.CredentialHolder, false, true)
		for _, connConfig := range allRepresentativeRegionConnections.Connectionconfig {
			if strings.EqualFold(req.ProviderName, connConfig.ProviderName) {
				verified := false
				for _, verifiedConnConfig := range verifiedConnections.Connectionconfig {
					if strings.EqualFold(connConfig.ConfigName, verifiedConnConfig.ConfigName) {
						verified = true
					}
				}
				// update representative regionZone with the verified regionZone
				if !verified {
					for _, verifiedConnConfig := range verifiedConnections.Connectionconfig {
						if strings.HasPrefix(verifiedConnConfig.ConfigName, connConfig.ConfigName) {
							connConfig.RegionZoneInfoName = verifiedConnConfig.RegionZoneInfoName
							connConfig.RegionZoneInfo = verifiedConnConfig.RegionZoneInfo
							break
						}
					}
					// update DB
					key := GenConnectionKey(connConfig.ConfigName)
					val, err := json.Marshal(connConfig)
					if err != nil {
						return callResult, err
					}
					err = kvstore.Put(string(key), string(val))
					if err != nil {
						return callResult, err
					}
				}
			}
		}
	}

	callResult.AllConnections, err = GetConnConfigList(req.CredentialHolder, false, false)
	if err != nil {
		log.Error().Err(err).Msg("")
		return callResult, err
	}

	return callResult, nil
}

// RegisterConnectionConfig is func to register connection config to CB-Spider
func RegisterConnectionConfig(connConfig model.ConnConfig) (model.ConnConfig, error) {
	client := resty.New()
	url := model.SpiderRestUrl + "/connectionconfig"
	method := "POST"
	var callResult model.SpiderConnConfig
	requestBody := model.SpiderConnConfig{}
	requestBody.ConfigName = connConfig.ConfigName
	requestBody.ProviderName = connConfig.ProviderName
	requestBody.DriverName = connConfig.DriverName
	requestBody.CredentialName = connConfig.CredentialName
	requestBody.RegionName = connConfig.RegionZoneInfoName

	err := ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestBody),
		&requestBody,
		&callResult,
		MediumDuration,
	)

	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ConnConfig{}, err
	}

	// Register connection to cb-tumblebug with availability check
	// verified, err := CheckConnConfigAvailable(callResult.ConfigName)
	// if err != nil {
	// 	log.Error().Err(err).Msgf("Cannot check model.ConnConfig %s is available", connConfig.ConfigName)
	// }
	// callResult.ProviderName = strings.ToLower(callResult.ProviderName)
	// if verified {
	// 	nativeRegion, _, err := GetRegion(callResult.RegionName)
	// 	if err != nil {
	// 		log.Error().Err(err).Msgf("Cannot get region for %s", callResult.RegionName)
	// 		callResult.Verified = false
	// 	} else {
	// 		location, err := GetCloudLocation(callResult.ProviderName, nativeRegion)
	// 		if err != nil {
	// 			log.Error().Err(err).Msgf("Cannot get location for %s/%s", callResult.ProviderName, nativeRegion)
	// 		}
	// 		callResult.Location = location
	// 	}
	// }

	connection := model.ConnConfig{}
	connection.ConfigName = callResult.ConfigName
	connection.ProviderName = strings.ToLower(callResult.ProviderName)
	connection.DriverName = callResult.DriverName
	connection.CredentialName = callResult.CredentialName
	connection.RegionZoneInfoName = callResult.RegionName
	connection.CredentialHolder = connConfig.CredentialHolder

	// load region info
	url = model.SpiderRestUrl + "/region/" + connection.RegionZoneInfoName
	method = "GET"
	var callResultRegion model.SpiderRegionZoneInfo
	requestNoBody := NoBody

	err = ExecuteHttpRequest(
		client,
		method,
		url,
		nil,
		SetUseBody(requestNoBody),
		&requestNoBody,
		&callResultRegion,
		MediumDuration,
	)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ConnConfig{}, err
	}
	regionZoneInfo := model.RegionZoneInfo{}
	for _, keyVal := range callResultRegion.KeyValueInfoList {
		if keyVal.Key == "Region" {
			regionZoneInfo.AssignedRegion = keyVal.Value
		}
		if keyVal.Key == "Zone" {
			regionZoneInfo.AssignedZone = keyVal.Value
		}
	}
	connection.RegionZoneInfo = regionZoneInfo

	regionDetail, err := GetRegion(connection.ProviderName, connection.RegionZoneInfo.AssignedRegion)
	if err != nil {
		log.Error().Err(err).Msgf("Cannot get region for %s", connection.RegionZoneInfo.AssignedRegion)
		return model.ConnConfig{}, err
	}
	connection.RegionDetail = regionDetail

	key := GenConnectionKey(connection.ConfigName)
	val, err := json.Marshal(connection)
	if err != nil {
		return model.ConnConfig{}, err
	}
	err = kvstore.Put(string(key), string(val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.ConnConfig{}, err
	}

	return connection, nil
}

// GetRegion is func to get regionInfo with the native region name
func GetRegion(ProviderName, RegionName string) (model.RegionDetail, error) {

	ProviderName = strings.ToLower(ProviderName)
	RegionName = strings.ToLower(RegionName)

	cloudInfo, err := GetCloudInfo()
	if err != nil {
		return model.RegionDetail{}, err
	}

	cspDetail, ok := cloudInfo.CSPs[ProviderName]
	if !ok {
		return model.RegionDetail{}, fmt.Errorf("cloudType '%s' not found", ProviderName)
	}

	// directly getting value from the map is disabled because of some possible case mismatches (enhancement needed)
	// regionDetail, ok := cspDetail.Regions[nativeRegion]
	// if !ok {
	// 	model.RegionDetail{}, fmt.Errorf("nativeRegion '%s' not found in Provider '%s'", RegionName, ProviderName)
	// }
	for key, regionDetail := range cspDetail.Regions {
		if strings.EqualFold(RegionName, key) {
			return regionDetail, nil
		}
	}

	return model.RegionDetail{}, fmt.Errorf("nativeRegion '%s' not found in Provider '%s'", RegionName, ProviderName)
}

// GetRegionList is func to retrieve region list
func GetRegionList() (model.RegionList, error) {

	url := model.SpiderRestUrl + "/region"

	client := resty.New().SetCloseConnection(true)

	resp, err := client.R().
		SetResult(&model.RegionList{}).
		//SetError(&SimpleMsg{}).
		Get(url)

	if err != nil {
		log.Error().Err(err).Msg("")
		content := model.RegionList{}
		err := fmt.Errorf("an error occurred while requesting to CB-Spider")
		return content, err
	}

	switch {
	case resp.StatusCode() >= 400 || resp.StatusCode() < 200:
		fmt.Println(" - HTTP Status: " + strconv.Itoa(resp.StatusCode()) + " in " + GetFuncName())
		err := fmt.Errorf(string(resp.Body()))
		log.Error().Err(err).Msg("")
		content := model.RegionList{}
		return content, err
	}

	temp, _ := resp.Result().(*model.RegionList)
	return *temp, nil

}

// GetCloudInfo is func to get all cloud info from the asset
func GetCloudInfo() (model.CloudInfo, error) {
	return RuntimeCloudInfo, nil
}

// ConvertToMessage is func to change input data to gRPC message
func ConvertToMessage(inType string, inData string, obj interface{}) error {
	//logger := logging.NewLogger()

	if inType == "yaml" {
		err := yaml.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		//logger.Debug("yaml Unmarshal: \n", obj)
	}

	if inType == "json" {
		err := json.Unmarshal([]byte(inData), obj)
		if err != nil {
			return err
		}
		//logger.Debug("json Unmarshal: \n", obj)
	}

	return nil
}

// ConvertToOutput is func to convert gRPC message to print format
func ConvertToOutput(outType string, obj interface{}) (string, error) {
	//logger := logging.NewLogger()

	if outType == "yaml" {
		// marshal using JSON to remove fields with XXX prefix
		j, err := json.Marshal(obj)
		if err != nil {
			return "", err
		}

		// use MapSlice to avoid sorting fields
		jsonObj := yaml.MapSlice{}
		err2 := yaml.Unmarshal(j, &jsonObj)
		if err2 != nil {
			return "", err2
		}

		// yaml marshal
		y, err3 := yaml.Marshal(jsonObj)
		if err3 != nil {
			return "", err3
		}
		//logger.Debug("yaml Marshal: \n", string(y))

		return string(y), nil
	}

	if outType == "json" {
		j, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return "", err
		}
		//logger.Debug("json Marshal: \n", string(j))

		return string(j), nil
	}

	return "", nil
}

// CopySrcToDest is func to copy data from source to target
func CopySrcToDest(src interface{}, dest interface{}) error {
	//logger := logging.NewLogger()

	j, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	//logger.Debug("source value : \n", string(j))

	err = json.Unmarshal(j, dest)
	if err != nil {
		return err
	}

	j, err = json.MarshalIndent(dest, "", "  ")
	if err != nil {
		return err
	}
	//logger.Debug("target value : \n", string(j))

	return nil
}

// NVL is func for null value logic
func NVL(str string, def string) string {
	if len(str) == 0 {
		return def
	}
	return str
}

// GetChildIdList is func to get child id list from given key
func GetChildIdList(key string) []string {

	keyValue, _ := kvstore.GetKvList(key)
	keyValue = kvutil.FilterKvListBy(keyValue, key, 1)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, strings.TrimPrefix(v.Key, key+"/"))

	}
	for _, v := range childIdList {
		fmt.Println("<" + v + "> \n")
	}

	return childIdList

}

// GetObjectList is func to return IDs of each child objects that has the same key
func GetObjectList(key string) []string {

	keyValue, _ := kvstore.GetKvList(key)

	var childIdList []string
	for _, v := range keyValue {
		childIdList = append(childIdList, v.Key)
	}

	return childIdList

}

// GetObjectValue is func to return the object value
func GetObjectValue(key string) (string, error) {

	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	if keyValue == (kvstore.KeyValue{}) {
		return "", nil
	}
	return keyValue.Value, nil
}

// DeleteObject is func to delete the object
func DeleteObject(key string) error {

	err := kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	return nil
}

// DeleteObjects is func to delete objects
func DeleteObjects(key string) error {
	keyValue, _ := kvstore.GetKvList(key)
	for _, v := range keyValue {
		err := kvstore.Delete(v.Key)
		if err != nil {
			log.Error().Err(err).Msg("")
			return err
		}
	}
	return nil
}

func CheckElement(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

const (
	// Random string generation
	letterBytes   = "abcdefghijklmnopqrstuvwxyz1234567890"
	letterIdxBits = 6
	letterIdxMask = 1<<letterIdxBits - 1
	letterIdxMax  = 63 / letterIdxBits
)

/* generate a random string (from CB-MCKS source code) */
func GenerateNewRandomString(n int) string {
	randSrc := rand.NewSource(time.Now().UnixNano()) //Random source by nano time
	b := make([]byte, n)
	for i, cache, remain := n-1, randSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = randSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

// GetK8sClusterInfo is func to get all kubernetes cluster info from the asset
func GetK8sClusterInfo() (model.K8sClusterInfo, error) {
	return RuntimeK8sClusterInfo, nil
}

func getK8sClusterDetail(providerName string) *model.K8sClusterDetail {
	// Get model.K8sClusterDetail for providerName
	var k8sClusterDetail *model.K8sClusterDetail = nil
	for provider, detail := range RuntimeK8sClusterInfo.CSPs {
		provider = strings.ToLower(provider)
		if provider == providerName {
			k8sClusterDetail = &detail
			break
		}
	}

	return k8sClusterDetail
}

// GetAvailableK8sClusterVersion is func to get available kubernetes cluster versions for provider and region from model.K8sClusterInfo
func GetAvailableK8sClusterVersion(providerName string, regionName string) (*[]model.K8sClusterVersionDetailAvailable, error) {
	//
	// Check available K8sCluster version in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)
	regionName = strings.ToLower(regionName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Check if 'regionName' exists
	var availableVersion *[]model.K8sClusterVersionDetailAvailable = nil
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, regionName) {
				if len(versionDetail.Available) == 0 {
					availableVersion = &[]model.K8sClusterVersionDetailAvailable{{model.StrEmpty, model.StrEmpty}}
				} else {
					availableVersion = &versionDetail.Available
				}
				return availableVersion, nil
			}
		}
	}

	// Check if 'common' exists
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, model.StrCommon) {
				if len(versionDetail.Available) == 0 {
					availableVersion = &[]model.K8sClusterVersionDetailAvailable{{model.StrEmpty, model.StrEmpty}}
				} else {
					availableVersion = &versionDetail.Available
				}
				return availableVersion, nil
			}
		}
	}

	return nil, fmt.Errorf("no entry for provider(%s):region(%s)", providerName, regionName)
}

// GetAvailableK8sClusterNodeImage is func to get available kubernetes cluster node images for provider and region from model.K8sClusterInfo
func GetAvailableK8sClusterNodeImage(providerName string, regionName string) (*[]model.K8sClusterNodeImageDetailAvailable, error) {
	//
	// Check available K8sCluster node image in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)
	regionName = strings.ToLower(regionName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	// Check if 'regionName' exists
	var availableNodeImage *[]model.K8sClusterNodeImageDetailAvailable = nil
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, regionName) {
				if len(nodeImageDetail.Available) == 0 {
					availableNodeImage = &[]model.K8sClusterNodeImageDetailAvailable{{model.StrEmpty, model.StrEmpty}}
					break
				} else {
					availableNodeImage = &nodeImageDetail.Available
				}
				return availableNodeImage, nil
			}
		}
	}

	// Check if 'common' exists
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if strings.EqualFold(region, model.StrCommon) {
				if len(nodeImageDetail.Available) == 0 {
					availableNodeImage = &[]model.K8sClusterNodeImageDetailAvailable{{model.StrEmpty, model.StrEmpty}}
					break
				} else {
					availableNodeImage = &nodeImageDetail.Available
				}
				return availableNodeImage, nil
			}
		}
	}

	return nil, fmt.Errorf("no available kubernetes cluster node image for region(%s) of provider(%s)", regionName, providerName)
}

// CheckNodeGroupsOnK8sCreation is func to check whether nodegroups are required during the k8scluster creation
func CheckNodeGroupsOnK8sCreation(providerName string) (*model.K8sClusterNodeGroupsOnCreation, error) {
	//
	// Check nodeGroupsOnCreation field in k8sclusterinfo.yaml
	//

	providerName = strings.ToLower(providerName)

	// Get model.K8sClusterDetail for providerName
	k8sClusterDetail := getK8sClusterDetail(providerName)
	if k8sClusterDetail == nil {
		return nil, fmt.Errorf("unsupported provider(%s) for kubernetes cluster", providerName)
	}

	return &model.K8sClusterNodeGroupsOnCreation{
		Result: strconv.FormatBool(k8sClusterDetail.NodeGroupsOnCreation),
	}, nil
}

/*
func isValidSpecForK8sCluster(spec *resource.TbSpecInfo) bool {
	//
	// Check for Provider
	//

	providerName := strings.ToLower(spec.ProviderName)

	var k8sClusterDetail *common.model.K8sClusterDetail = nil
	for provider, detail := range common.RuntimeK8sClusterInfo.CSPs {
		provider = strings.ToLower(provider)
		if provider == providerName {
			k8sClusterDetail = &detail
			break
		}
	}
	if k8sClusterDetail == nil {
		return false
	}

	//
	// Check for Region
	//

	regionName := strings.ToLower(spec.RegionName)

	// Check for Version
	isExist := false
	for _, versionDetail := range k8sClusterDetail.Version {
		for _, region := range versionDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				if len(versionDetail.Available) > 0 {
					isExist = true
					break
				}
			}
		}
		if isExist == true {
			break
		}
	}
	if isExist == false {
		return false
	}

	// Check for NodeImage
	isExist = false
	for _, nodeImageDetail := range k8sClusterDetail.NodeImage {
		for _, region := range nodeImageDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				if len(nodeImageDetail.Available) > 0 {
					isExist = true
					break
				}
			}
		}
		if isExist == true {
			break
		}
	}
	if isExist == false {
		return false
	}

	// Check for RootDisk
	isExist = false
	for _, rootDiskDetail := range k8sClusterDetail.RootDisk {
		for _, region := range rootDiskDetail.Region {
			region = strings.ToLower(region)
			if region == "all" || region == regionName {
				if len(rootDiskDetail.Type) > 0 {
					isExist = true
					break
				}
			}
		}
		if isExist == true {
			break
		}
	}
	if isExist == false {
		return false
	}

	return true
}
*/
