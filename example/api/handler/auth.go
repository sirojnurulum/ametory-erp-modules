package handler

import (
	"ametory-erp/config"
	"net/http"
	"time"

	"github.com/AMETORY/ametory-erp-modules/app"
	"github.com/AMETORY/ametory-erp-modules/utils"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	appContainer *app.AppContainer
}

func NewAuthHandler(appContainer *app.AppContainer) *AuthHandler {
	return &AuthHandler{
		appContainer: appContainer,
	}
}

// Login handles a login request and returns a JWT token if the credentials are valid.
// The token is valid for the duration specified by Server.TokenExpiredDay in the configuration file.
// The response will be a JSON object with a single key "token" containing the JWT token.
func (h *AuthHandler) Login(c *gin.Context) {
	type LoginInput struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Implement login logic here, e.g., authenticate user and return a token
	user, err := h.appContainer.AuthService.Login(input.Username, input.Password, false)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := utils.GenerateJWT(user.ID, time.Now().AddDate(0, 0, config.App.Server.TokenExpiredDay).Unix(), config.App.Server.SecretKey)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "token": token})
}
