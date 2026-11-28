package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"xiaopi/backend/internal/domain"
)

const (
	defaultAreaListingTemplate = "bNiSZR7Pm2YbN2IwackAtruc0_qielqi9SAQluAGJu4"
	defaultAppointmentTemplate = "A4aTh-uYI__506njzEHICAKLM5KlvLDVrRpwAkQJagk"
)

type messageSubscriptionStore interface {
	AddMessageSubscriptionGrant(context.Context, string, string, string) error
	ListAdminMessageRecipients(context.Context, string) ([]domain.MessageRecipient, error)
	ListAreaMessageRecipients(context.Context, string, domain.Property) ([]domain.MessageRecipient, error)
	ConsumeMessageSubscriptionGrant(context.Context, uint64) error
	ListUnnotifiedRentalRequests(context.Context, string) ([]domain.RentalRequest, error)
	MarkRentalRequestAdminNotified(context.Context, string) error
}

func (s *Server) messageStore() (messageSubscriptionStore, bool) {
	store, ok := s.store.(messageSubscriptionStore)
	return store, ok
}
func areaTemplateID() string {
	return envOr("WECHAT_AREA_LISTING_TEMPLATE_ID", defaultAreaListingTemplate)
}
func appointmentTemplateID() string {
	return envOr("WECHAT_APPOINTMENT_TEMPLATE_ID", defaultAppointmentTemplate)
}
func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func (s *Server) subscriptionTemplates(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": map[string]string{"areaListing": areaTemplateID(), "appointment": appointmentTemplateID()}})
}

func (s *Server) registerMessageSubscription(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TemplateID string `json:"templateId"`
		Area       string `json:"area"`
	}
	if decodeJSON(w, r, &input) != nil {
		writeError(w, http.StatusBadRequest, "invalid subscription")
		return
	}
	input.TemplateID, input.Area = strings.TrimSpace(input.TemplateID), strings.TrimSpace(input.Area)
	if input.TemplateID != areaTemplateID() && input.TemplateID != appointmentTemplateID() {
		writeError(w, http.StatusBadRequest, "订阅模板无效")
		return
	}
	if input.TemplateID == appointmentTemplateID() && s.effectiveRole(r.Context(), claimsFromContext(r.Context())) != roleAdmin {
		writeError(w, http.StatusForbidden, "仅管理员可以订阅预约提醒")
		return
	}
	if input.TemplateID == areaTemplateID() && input.Area == "" {
		writeError(w, http.StatusBadRequest, "订阅片区不能为空")
		return
	}
	store, ok := s.messageStore()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "message subscription unavailable")
		return
	}
	if err := store.AddMessageSubscriptionGrant(r.Context(), claimsFromContext(r.Context()).Subject, input.TemplateID, input.Area); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save subscription")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"code": 0, "message": "success"})
}

func shortText(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
func messageValue(value string) map[string]string { return map[string]string{"value": value} }

func (s *Server) notifyAdminsOfRequest(ctx context.Context, item domain.RentalRequest, jobID uint64) error {
	if item.Phone == "" {
		return nil
	}
	store, ok := s.messageStore()
	if !ok {
		return nil
	}
	recipients, err := store.ListAdminMessageRecipients(ctx, appointmentTemplateID())
	if paged, ok := s.store.(interface {
		ListAdminMessageRecipientsForJob(context.Context, string, uint64) ([]domain.MessageRecipient, error)
	}); ok && jobID != 0 {
		recipients, err = paged.ListAdminMessageRecipientsForJob(ctx, appointmentTemplateID(), jobID)
	}
	if err != nil {
		return err
	}
	typeName := map[string]string{"appointment": "预约看房", "area_subscription": "片区新房预约", "general": "找房需求"}[item.RequestType]
	location := item.Location
	if location == "" {
		location = "指定房源"
	}
	note := shortText(item.Note, 20)
	if note == "" {
		note = "用户已提交预约"
	}
	data := map[string]any{"thing11": messageValue(shortText(location, 20)), "thing12": messageValue(typeName), "phone_number14": messageValue(item.Phone), "date5": messageValue(time.Now().Format("2006-01-02")), "thing6": messageValue(note)}
	var retry error
	if jobID != 0 && len(recipients) > 0 {
		retry = errSubscriptionBatch
	}
	for _, recipient := range recipients {
		sent, err := s.deliverSubscription(ctx, store, jobID, recipient, func() error {
			return s.sendSubscribeMessage(ctx, recipient.OpenID, appointmentTemplateID(), "pages/admin/index", data)
		})
		if sent && err == nil {
			_ = store.MarkRentalRequestAdminNotified(ctx, item.ID)
			s.logOperation(ctx, recipient.UserID, "subscription", item.ID, "预约提醒已发送", typeName)
		} else if err != nil {
			retry = err
			s.logOperation(ctx, recipient.UserID, "subscription", item.ID, "预约提醒发送失败", shortText(err.Error(), 100))
		}
	}
	return retry
}

func (s *Server) notifyAreaSubscribers(ctx context.Context, property domain.Property, jobID uint64) error {
	store, ok := s.messageStore()
	if !ok {
		return nil
	}
	recipients, err := store.ListAreaMessageRecipients(ctx, areaTemplateID(), property)
	if paged, ok := s.store.(interface {
		ListAreaMessageRecipientsForJob(context.Context, string, domain.Property, uint64) ([]domain.MessageRecipient, error)
	}); ok && jobID != 0 {
		recipients, err = paged.ListAreaMessageRecipientsForJob(ctx, areaTemplateID(), property, jobID)
	}
	if err != nil {
		return err
	}
	var retry error
	if jobID != 0 && len(recipients) > 0 {
		retry = errSubscriptionBatch
	}
	for _, recipient := range recipients {
		payment := property.FeeText
		if payment == "" {
			payment = "以房源详情为准"
		}
		layout := shortText(property.Layout, 20)
		if layout == "" {
			layout = "户型详见房源"
		}
		data := map[string]any{"thing2": messageValue(shortText(property.Community, 20)), "thing1": messageValue(shortText(recipient.Area, 20)), "amount4": messageValue(fmt.Sprintf("%d元", property.RentMonthly)), "thing5": messageValue(shortText(payment, 20)), "thing11": messageValue(layout)}
		sent, err := s.deliverSubscription(ctx, store, jobID, recipient, func() error {
			return s.sendSubscribeMessage(ctx, recipient.OpenID, areaTemplateID(), "pages/properties/detail?id="+url.QueryEscape(property.ID), data)
		})
		if sent && err == nil {
			s.logOperation(ctx, recipient.UserID, "subscription", property.ID, "片区上新提醒已发送", recipient.Area)
		} else if err != nil {
			retry = err
			s.logOperation(ctx, recipient.UserID, "subscription", property.ID, "片区上新提醒发送失败", shortText(err.Error(), 100))
		}
	}
	return retry
}

func (s *Server) sendSubscribeMessage(ctx context.Context, openID, templateID, page string, data map[string]any) error {
	appID, secret := os.Getenv("WECHAT_MINI_APP_ID"), os.Getenv("WECHAT_MINI_APP_SECRET")
	if appID == "" || secret == "" {
		return &domain.SubscriptionNotSent{Cause: errors.New("wechat not configured")}
	}
	tokenURL := "https://api.weixin.qq.com/cgi-bin/token?" + url.Values{"grant_type": {"client_credential"}, "appid": {appID}, "secret": {secret}}.Encode()
	tokenRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return &domain.SubscriptionNotSent{Cause: errors.New("wechat token request invalid")}
	}
	response, err := s.httpClient.Do(tokenRequest)
	if err != nil {
		return &domain.SubscriptionNotSent{Cause: errors.New("wechat token request failed")}
	}
	defer response.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if json.NewDecoder(response.Body).Decode(&token) != nil || token.AccessToken == "" {
		return &domain.SubscriptionNotSent{Cause: errors.New("wechat token unavailable")}
	}
	payload, _ := json.Marshal(map[string]any{"touser": openID, "template_id": templateID, "page": page, "data": data, "miniprogram_state": envOr("WECHAT_MINIPROGRAM_STATE", "formal"), "lang": "zh_CN"})
	request, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.weixin.qq.com/cgi-bin/message/subscribe/send?access_token="+url.QueryEscape(token.AccessToken), bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	result, err := s.httpClient.Do(request)
	if err != nil {
		return errors.New("wechat subscription request failed")
	}
	defer result.Body.Close()
	var body struct {
		ErrCode *int   `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if result.StatusCode != http.StatusOK || json.NewDecoder(result.Body).Decode(&body) != nil || body.ErrCode == nil {
		return errors.New("wechat subscription response invalid")
	}
	if *body.ErrCode != 0 {
		return &domain.SubscriptionNotSent{
			Cause:         fmt.Errorf("wechat subscribe message: %d %s", *body.ErrCode, body.ErrMsg),
			InvalidGrant:  *body.ErrCode == 43101 || *body.ErrCode == 40003,
			Configuration: *body.ErrCode == 40037 || *body.ErrCode == 47003 || *body.ErrCode == 41030,
		}
	}
	return nil
}
