package httpapi

import (
	"encoding/json"
	"io"
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

// staticLocationMap returns a small server-proxied Tencent static map. The
// browser never receives the location-service key.
func (s *Server) staticLocationMap(w http.ResponseWriter, r *http.Request) {
	if s.config.TencentMapKey == "" {
		writeError(w, http.StatusServiceUnavailable, "地图服务尚未配置")
		return
	}
	latitude, longitude := queryFloat(r, "latitude", 0), queryFloat(r, "longitude", 0)
	if latitude == 0 || longitude == 0 || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		writeError(w, http.StatusBadRequest, "地图坐标不正确")
		return
	}
	query := url.Values{}
	query.Set("key", s.config.TencentMapKey)
	query.Set("center", strconv.FormatFloat(latitude, 'f', 6, 64)+","+strconv.FormatFloat(longitude, 'f', 6, 64))
	query.Set("zoom", "14")
	query.Set("size", "620*260")
	query.Set("scale", "2")
	query.Set("markers", "size:large|color:0x40543f|label:P|"+strconv.FormatFloat(latitude, 'f', 6, 64)+","+strconv.FormatFloat(longitude, 'f', 6, 64))
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.config.TencentMapURL+"/ws/staticmap/v2/?"+query.Encode(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "地图预览请求创建失败")
		return
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "地图预览暂时不可用")
		return
	}
	defer response.Body.Close()
	contentType := response.Header.Get("Content-Type")
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusBadGateway, "地图预览生成失败")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = io.Copy(w, io.LimitReader(response.Body, 5<<20))
}

func queryFloat(r *http.Request, name string, fallback float64) float64 {
	value, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
	if err != nil || value == 0 {
		return fallback
	}
	return value
}
