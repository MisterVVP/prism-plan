package main

import (
	"encoding/json"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/labstack/echo/v4"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type userEntity struct {
	aztables.Entity
	Data string `json:"Data"`
}

func postUser(c echo.Context) error {
	ctx := c.Request().Context()
	userID, err := userIDFromAuthHeader(c.Request().Header.Get("Authorization"))
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	var u User
	if err := c.Bind(&u); err != nil {
		return c.String(http.StatusBadRequest, "invalid body")
	}
	u.ID = userID
	data, _ := json.Marshal(u)
	ent := map[string]interface{}{
		"PartitionKey": "user",
		"RowKey":       userID,
		"Data":         string(data),
	}
	payload, _ := json.Marshal(ent)
	userClient.UpsertEntity(ctx, payload, nil)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
