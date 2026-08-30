package httpapi

import (
	"net/http"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

func (h *Handler) login(response http.ResponseWriter, request *http.Request) {
	var payload loginRequest
	if err := h.decodeJSON(response, request, &payload); err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.auth.Login(payload.Username, payload.Password, payload.Role)
	if err != nil {
		h.writeError(response, http.StatusUnauthorized, "Invalid username or password", nil)
		return
	}
	h.writeJSON(response, http.StatusOK, authDTO(result))
}

func (h *Handler) currentUser(response http.ResponseWriter, request *http.Request) {
	claims, ok := requestClaims(request)
	if !ok {
		if h.requireAuth {
			h.writeError(response, http.StatusUnauthorized, "A valid bearer session is required", nil)
			return
		}
		// Explicit demo mode keeps the legacy local current-user response.
		claims = service.AuthClaims{UserID: "ramanred", Role: "SRE_LEAD"}
	}
	h.writeJSON(response, http.StatusOK, authDTO(h.auth.CurrentUser(claims)))
}

func authDTO(value service.AuthResponse) authResponse {
	return authResponse{
		Authenticated: value.Authenticated, Token: value.Token, UserID: value.UserID,
		Username: value.Username, Email: value.Email, Role: value.Role,
		AvatarURL: value.AvatarURL, Message: value.Message,
	}
}
