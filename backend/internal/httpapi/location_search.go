package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
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

type staticMapPoint struct {
	Latitude  float64
	Longitude float64
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
	latitude, longitude := queryFloat(r, "latitude", 30.45604), queryFloat(r, "longitude", 114.425337)
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		writeError(w, http.StatusBadRequest, "地图坐标不正确")
		return
	}
	points := parseStaticMapPoints(r.URL.Query().Get("points"))
	if len(points) > 0 {
		var sumLatitude, sumLongitude float64
		for _, point := range points {
			sumLatitude += point.Latitude
			sumLongitude += point.Longitude
		}
		latitude = sumLatitude / float64(len(points))
		longitude = sumLongitude / float64(len(points))
	}
	cacheKey := strconv.FormatFloat(latitude, 'f', 6, 64) + "," + strconv.FormatFloat(longitude, 'f', 6, 64) + "|" + r.URL.Query().Get("points")
	s.staticMapMu.Lock()
	cached, found := s.staticMaps[cacheKey]
	if found && time.Now().Before(cached.ExpiresAt) {
		s.staticMapMu.Unlock()
		w.Header().Set("Content-Type", cached.ContentType)
		w.Header().Set("Cache-Control", "private, max-age=300")
		_, _ = w.Write(cached.Body)
		return
	}
	s.staticMapMu.Unlock()
	query := url.Values{}
	query.Set("key", s.config.TencentMapKey)
	query.Set("center", strconv.FormatFloat(latitude, 'f', 6, 64)+","+strconv.FormatFloat(longitude, 'f', 6, 64))
	if len(points) > 1 {
		query.Set("zoom", "12")
	} else {
		query.Set("zoom", "14")
	}
	query.Set("size", "620*260")
	query.Set("scale", "2")
	markers := make([]string, 0, max(1, len(points)))
	if len(points) == 0 {
		markers = append(markers, "size:large|color:0x40543f|label:P|"+strconv.FormatFloat(latitude, 'f', 6, 64)+","+strconv.FormatFloat(longitude, 'f', 6, 64))
	} else {
		for index, point := range points {
			markers = append(markers, "size:medium|color:0xd76440|label:"+string(rune('A'+index))+"|"+strconv.FormatFloat(point.Latitude, 'f', 6, 64)+","+strconv.FormatFloat(point.Longitude, 'f', 6, 64))
		}
	}
	query.Set("markers", strings.Join(markers, ";"))
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
	body, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil || len(body) == 0 {
		writeError(w, http.StatusBadGateway, "地图预览读取失败")
		return
	}
	s.staticMapMu.Lock()
	s.staticMaps[cacheKey] = staticMapCacheEntry{Body: body, ContentType: contentType, ExpiresAt: time.Now().Add(10 * time.Minute)}
	s.staticMapMu.Unlock()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(body)
}

// parseStaticMapPoints accepts up to eight "latitude,longitude" pairs. They
// are supplied by the admin UI after a POI search and never expose the map key.
func parseStaticMapPoints(raw string) []staticMapPoint {
	points := make([]staticMapPoint, 0, 8)
	for _, value := range strings.Split(raw, ";") {
		if len(points) == 8 {
			break
		}
		parts := strings.Split(strings.TrimSpace(value), ",")
		if len(parts) != 2 {
			continue
		}
		latitude, latitudeErr := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		longitude, longitudeErr := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if latitudeErr != nil || longitudeErr != nil || latitude == 0 || longitude == 0 || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
			continue
		}
		points = append(points, staticMapPoint{Latitude: latitude, Longitude: longitude})
	}
	return points
}

func queryFloat(r *http.Request, name string, fallback float64) float64 {
	value, err := strconv.ParseFloat(r.URL.Query().Get(name), 64)
	if err != nil || value == 0 {
		return fallback
	}
	return value
}
