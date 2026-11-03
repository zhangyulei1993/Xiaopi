package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type locationSearchItem struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Address   string  `json:"address"`
	District  string  `json:"district"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Distance  float64 `json:"distance,omitempty"`
}

func (s *Server) searchLocations(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if len([]rune(keyword)) < 2 || len([]rune(keyword)) > 32 {
		writeError(w, http.StatusBadRequest, "请输入2到32个字的小区或街道名称")
		return
	}
	if s.config.TencentMapKey == "" {
		writeError(w, http.StatusServiceUnavailable, "地图搜索服务尚未配置")
		return
	}

	latitude := queryFloat(r, "latitude", 30.45604)
	longitude := queryFloat(r, "longitude", 114.425337)
	query := url.Values{}
	query.Set("key", s.config.TencentMapKey)
	query.Set("keyword", keyword)
	query.Set("boundary", "region(武汉,1,"+strconv.FormatFloat(latitude, 'f', 6, 64)+","+strconv.FormatFloat(longitude, 'f', 6, 64)+")")
	query.Set("page_size", "8")
	query.Set("page_index", "1")
	query.Set("orderby", "_distance")

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.config.TencentMapURL+"/ws/place/v1/search?"+query.Encode(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "地图搜索请求创建失败")
		return
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "地图搜索服务暂时不可用")
		return
	}
	defer response.Body.Close()

	var result struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
		Data    []struct {
			ID       string  `json:"id"`
			Title    string  `json:"title"`
			Address  string  `json:"address"`
			Distance float64 `json:"_distance"`
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
			AdInfo struct {
				District string `json:"district"`
			} `json:"ad_info"`
		} `json:"data"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&result) != nil {
		writeError(w, http.StatusBadGateway, "地点搜索失败，请稍后重试")
		return
	}
	if result.Status != 0 {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "地点搜索失败，请稍后重试"
		}
		writeError(w, http.StatusBadGateway, message)
		return
	}

	items := make([]locationSearchItem, 0, len(result.Data))
	for _, item := range result.Data {
		if item.Location.Lat == 0 || item.Location.Lng == 0 {
			continue
		}
		items = append(items, locationSearchItem{
			ID: item.ID, Title: item.Title, Address: item.Address, District: item.AdInfo.District,
			Latitude: item.Location.Lat, Longitude: item.Location.Lng, Distance: item.Distance,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "message": "success", "data": items})
}

func queryFloat(r *http.Request, name string, fallback float64) float64 {
	value, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
	if err != nil || value == 0 {
		return fallback
	}
	return value
}
