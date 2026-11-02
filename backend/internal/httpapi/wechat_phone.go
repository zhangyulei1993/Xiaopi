package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"xiaopi/backend/internal/domain"
)

func (s *Server) bindWechatPhone(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if decodeJSON(w, r, &input) != nil || strings.TrimSpace(input.Code) == "" {
		writeError(w, http.StatusBadRequest, "微信手机号授权码不能为空")
		return
	}
	store, ok := s.userAdminStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "user store unavailable")
		return
	}
	phone, err := s.phoneNumberFromWechat(input.Code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "微信手机号获取失败，请重新授权")
		return
	}
	userID := claimsFromContext(r.Context()).Subject
	if err := store.UpdateUserPhone(r.Context(), userID, phone); errors.Is(err, domain.ErrPhoneInUse) {
		writeError(w, http.StatusConflict, "该手机号已绑定其他微信用户")
		return
	} else if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "微信用户不存在，请重新登录")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to bind phone")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"phone": phone}})
}

func (s *Server) phoneNumberFromWechat(code string) (string, error) {
	appID, secret := os.Getenv("WECHAT_MINI_APP_ID"), os.Getenv("WECHAT_MINI_APP_SECRET")
	if appID == "" || secret == "" {
		return "", errors.New("wechat authentication is not configured")
	}
	tokenURL := "https://api.weixin.qq.com/cgi-bin/token?" + url.Values{"grant_type": {"client_credential"}, "appid": {appID}, "secret": {secret}}.Encode()
	response, err := s.httpClient.Get(tokenURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var tokenResult struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if json.NewDecoder(response.Body).Decode(&tokenResult) != nil || tokenResult.AccessToken == "" {
		return "", errors.New("wechat access token unavailable")
	}
	payload, _ := json.Marshal(map[string]string{"code": code})
	request, err := http.NewRequest(http.MethodPost, "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(tokenResult.AccessToken), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	phoneResponse, err := s.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer phoneResponse.Body.Close()
	var phoneResult struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber     string `json:"phoneNumber"`
			PurePhoneNumber string `json:"purePhoneNumber"`
		} `json:"phone_info"`
	}
	if json.NewDecoder(phoneResponse.Body).Decode(&phoneResult) != nil || phoneResult.ErrCode != 0 {
		return "", errors.New("wechat phone unavailable")
	}
	phone := normalizePhone(phoneResult.PhoneInfo.PurePhoneNumber)
	if phone == "" {
		phone = normalizePhone(phoneResult.PhoneInfo.PhoneNumber)
	}
	if phone == "" {
		return "", errors.New("wechat returned empty phone")
	}
	return phone, nil
}
