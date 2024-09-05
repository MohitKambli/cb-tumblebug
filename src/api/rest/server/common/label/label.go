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

// Package label is to handle label selector for resources
package label

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/common/label"
)

// RestCreateOrUpdateLabel godoc
// @ID CreateOrUpdateLabel
// @Summary Create or update a label for a resource
// @Description Create or update a label for a resource identified by its uid
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param labelType path string true "Label Type (e.g., ns, vnet)"
// @Param uid path string true "Resource uid"
// @Param labels body map[string]string true "Labels to create or update"
// @Success 200 {object} model.SimpleMsg "Label created or updated successfully"
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /label/{labelType}/{uid} [put]
func RestCreateOrUpdateLabel(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	labelType := c.Param("labelType")
	uid := c.Param("uid")

	// Parse the incoming request body to get the labels
	labels := make(map[string]string)
	if err := c.Bind(&labels); err != nil {
		return common.EndRequestWithLog(c, reqID, fmt.Errorf("Invalid request body"), nil)
	}

	// Get the resource key
	resourceKey := fmt.Sprintf("/%s/%s", labelType, uid)

	// Create or update the label in the KV store
	err := label.CreateOrUpdateLabel(labelType, uid, resourceKey, labels)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	return common.EndRequestWithLog(c, reqID, nil, map[string]string{"message": "Label created or updated successfully"})
}

// RestRemoveLabel godoc
// @ID RemoveLabel
// @Summary Remove a label from a resource
// @Description Remove a label from a resource identified by its uid
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param labelType path string true "Label Type (e.g., ns, vnet)"
// @Param uid path string true "Resource uid"
// @Param key path string true "Label key to remove"
// @Success 200 {object} model.SimpleMsg "Label removed successfully"
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /label/{labelType}/{uid}/{key} [delete]
func RestRemoveLabel(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	labelType := c.Param("labelType")
	uid := c.Param("uid")
	key := c.Param("key")

	// Remove the label from the KV store
	err := label.RemoveLabel(labelType, uid, key)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	return common.EndRequestWithLog(c, reqID, nil, map[string]string{"message": "Label removed successfully"})
}

// RestGetLabels godoc
// @ID GetLabels
// @Summary Get labels for a resource
// @Description Get labels for a resource identified by its uid
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param labelType path string true "Label Type (e.g., ns, vnet)"
// @Param uid path string true "Resource uid"
// @Success 200 {object} map[string]string "Labels for the resource"
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /label/{labelType}/{uid} [get]
func RestGetLabels(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	labelType := c.Param("labelType")
	uid := c.Param("uid")

	// Get the labels from the KV store
	labelInfo, err := label.GetLabels(labelType, uid)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	return common.EndRequestWithLog(c, reqID, nil, labelInfo.Labels)
}

// ResourcesResponse is a struct to wrap the results of a label selector query
type ResourcesResponse struct {
	Results []interface{} `json:"results"`
}

// RestGetResourcesByLabelSelector godoc
// @ID GetResourcesByLabelSelector
// @Summary Get resources by label selector
// @Description Get resources based on a label selector. The label selector supports the following operators:
// @Description - `=` : Selects resources where the label key equals the specified value (e.g., `env=production`).
// @Description - `!=` : Selects resources where the label key does not equal the specified value (e.g., `tier!=frontend`).
// @Description - `in` : Selects resources where the label key is in the specified set of values (e.g., `region in (us-west, us-east)`).
// @Description - `notin` : Selects resources where the label key is not in the specified set of values (e.g., `env notin (production, staging)`).
// @Description - `exists` : Selects resources where the label key exists (e.g., `env exists`).
// @Description - `!exists` : Selects resources where the label key does not exist (e.g., `env !exists`).
// @Tags [Infra Resource] Common Utility
// @Accept  json
// @Produce  json
// @Param labelType path string true "Label Type (e.g., ns, sshKey, vNet, vm, mci, k8s, etc.)"
// @Param labelSelector query string true "Label selector query. Example: env=production,tier=backend"
// @Success 200 {object} ResourcesResponse "Matched resources"
// @Failure 400 {object} model.SimpleMsg "Invalid request"
// @Failure 500 {object} model.SimpleMsg "Internal Server Error"
// @Router /resources/{labelType} [get]
func RestGetResourcesByLabelSelector(c echo.Context) error {
	reqID, idErr := common.StartRequestWithLog(c)
	if idErr != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": idErr.Error()})
	}

	labelType := c.Param("labelType")
	labelSelector := c.QueryParam("labelSelector")

	// Get resources based on the label selector
	resources, err := label.GetResourcesByLabelSelector(labelType, labelSelector)
	if err != nil {
		return common.EndRequestWithLog(c, reqID, err, nil)
	}

	// Wrap the results in a JSON object
	response := ResourcesResponse{
		Results: resources,
	}

	return common.EndRequestWithLog(c, reqID, nil, response)
}