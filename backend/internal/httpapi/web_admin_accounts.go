package httpapi

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
)

const webAdminPrefix = "web-admin:"

var webAdminName = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)

type webAdminAccount struct {
	Username   string `json:"username"`
	Enabled    bool   `json:"enabled"`
	MustChange bool   `json:"mustChangePassword"`
	Super      bool   `json:"super"`
	hash       string
	version    string
}

func (s *Server) webAdminDB() *sql.DB {
	if store, ok := s.store.(interface{ DB() *sql.DB }); ok {
		return store.DB()
	}
	return nil
}

func passwordDigest(password string, salt []byte) string {
	key, _ := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(key)
}

func hashWebPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	return passwordDigest(password, salt)
}

func readWebAdmin(db *sql.DB, r *http.Request, name string) (a webAdminAccount, err error) {
	a.Username, a.Super = name, name == "admin"
	err = db.QueryRowContext(r.Context(), "SELECT enabled,must_change_password,password_hash,session_version FROM web_admin_accounts WHERE username=?", name).Scan(&a.Enabled, &a.MustChange, &a.hash, &a.version)
	return
}

func (s *Server) loginWebAdmin(w http.ResponseWriter, r *http.Request, db *sql.DB, name, password, key string) {
	if name == s.config.AdminUsername && name != "" {
		name = "admin"
	}
	if name == "admin" {
		initial := s.config.AdminPassword
		if initial == "" {
			initial = "adminadmin"
		}
		if _, err := readWebAdmin(db, r, name); err == sql.ErrNoRows {
			version, e := newID()
			if e != nil {
				writeError(w, 500, "无法初始化后台账号")
				return
			}
			if _, e = db.ExecContext(r.Context(), "INSERT INTO web_admin_accounts(username,password_hash,session_version,must_change_password) VALUES ('admin',?,?,?) ON DUPLICATE KEY UPDATE username=username", hashWebPassword(initial), version, initial == "adminadmin"); e != nil {
				writeError(w, 500, "无法初始化后台账号")
				return
			}
		}
	}
	a, err := readWebAdmin(db, r, name)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, 503, "后台账号服务不可用")
		return
	}
	saltHex, _, _ := strings.Cut(a.hash, ":")
	salt, _ := hex.DecodeString(saltHex)
	if len(salt) != 16 {
		salt = make([]byte, 16)
	}
	if !secureEqual(passwordDigest(password, salt), a.hash) || !a.Enabled {
		s.recordLoginFailure(key)
		writeError(w, 401, "账号或密码错误")
		return
	}
	s.clearLoginFailures(key)
	token, err := s.tokens.issue(webAdminPrefix+a.Username+":"+a.version, roleAdmin)
	if err != nil {
		writeError(w, 503, "后台认证未配置")
		return
	}
	webAdminReply(w, map[string]any{"username": a.Username, "accessToken": token, "mustChangePassword": a.MustChange, "super": a.Super})
}

func webAdminReply(w http.ResponseWriter, data any) {
	writeJSON(w, 200, map[string]any{"code": 0, "message": "success", "data": data})
}

func (s *Server) webAdminSession(r *http.Request, claims tokenClaims) (webAdminAccount, bool) {
	name, version, ok := strings.Cut(strings.TrimPrefix(claims.Subject, webAdminPrefix), ":")
	db := s.webAdminDB()
	if !ok || db == nil || claims.Role != roleAdmin {
		return webAdminAccount{}, false
	}
	a, err := readWebAdmin(db, r, name)
	return a, err == nil && a.Enabled && a.version == version
}

func (s *Server) manageWebAdmins(w http.ResponseWriter, r *http.Request) {
	claims, err := s.tokens.verify(bearerToken(r))
	if err != nil || !strings.HasPrefix(claims.Subject, webAdminPrefix) {
		writeError(w, 401, "请登录 Web 后台")
		return
	}
	actor, ok := s.webAdminSession(r, claims)
	if !ok {
		writeError(w, 401, "请重新登录")
		return
	}
	db := s.webAdminDB()
	if r.URL.Path == "/api/v1/admin/web-password" {
		var input struct {
			Password string `json:"password"`
		}
		if decodeJSON(w, r, &input) != nil || !validWebPassword(input.Password) {
			writeError(w, 400, "请输入 10 至 128 字节的新密码")
			return
		}
		version, e := newID()
		if e != nil {
			writeError(w, 500, "改密失败")
			return
		}
		result, e := db.ExecContext(r.Context(), "UPDATE web_admin_accounts SET password_hash=?,session_version=?,must_change_password=FALSE WHERE username=? AND session_version=?", hashWebPassword(input.Password), version, actor.Username, actor.version)
		if e != nil {
			writeError(w, 500, "改密失败")
			return
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			writeError(w, 401, "请重新登录")
			return
		}
		webAdminReply(w, nil)
		return
	}
	if !actor.Super {
		writeError(w, 403, "需要超级管理员权限")
		return
	}
	if r.Method == http.MethodGet {
		rows, e := db.QueryContext(r.Context(), "SELECT username,enabled,must_change_password FROM web_admin_accounts ORDER BY username")
		if e != nil {
			writeError(w, 500, "读取后台账号失败")
			return
		}
		defer rows.Close()
		items := []webAdminAccount{}
		for rows.Next() {
			var a webAdminAccount
			if rows.Scan(&a.Username, &a.Enabled, &a.MustChange) != nil {
				writeError(w, 500, "读取后台账号失败")
				return
			}
			a.Super = a.Username == "admin"
			items = append(items, a)
		}
		if rows.Err() != nil {
			writeError(w, 500, "读取后台账号失败")
			return
		}
		webAdminReply(w, items)
		return
	}
	name := r.PathValue("username")
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Enabled  *bool  `json:"enabled"`
	}
	if r.Method != http.MethodDelete && decodeJSON(w, r, &input) != nil {
		writeError(w, 400, "请求格式错误")
		return
	}
	if r.Method == http.MethodPost {
		name = input.Username
	}
	if !webAdminName.MatchString(name) || name == "admin" || name == s.config.AdminUsername {
		writeError(w, 400, "账号名无效或为系统保留账号")
		return
	}
	version, e := newID()
	if e != nil {
		writeError(w, 500, "操作失败")
		return
	}
	var result sql.Result
	switch r.Method {
	case http.MethodPost:
		if !validWebPassword(input.Password) {
			writeError(w, 400, "请输入 10 至 128 字节的初始密码")
			return
		}
		result, e = db.ExecContext(r.Context(), "INSERT IGNORE INTO web_admin_accounts(username,password_hash,session_version) VALUES (?,?,?)", name, hashWebPassword(input.Password), version)
	case http.MethodPatch:
		if input.Password != "" && !validWebPassword(input.Password) {
			writeError(w, 400, "请输入 10 至 128 字节的新密码")
			return
		}
		if input.Password != "" {
			result, e = db.ExecContext(r.Context(), "UPDATE web_admin_accounts SET enabled=COALESCE(?,enabled),password_hash=?,must_change_password=TRUE,session_version=? WHERE username=?", input.Enabled, hashWebPassword(input.Password), version, name)
		} else {
			result, e = db.ExecContext(r.Context(), "UPDATE web_admin_accounts SET enabled=COALESCE(?,enabled),session_version=? WHERE username=?", input.Enabled, version, name)
		}
	case http.MethodDelete:
		result, e = db.ExecContext(r.Context(), "DELETE FROM web_admin_accounts WHERE username=?", name)
	}
	if e != nil {
		writeError(w, 500, "操作失败")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		if r.Method == http.MethodPost {
			writeError(w, 409, "账号已存在")
		} else {
			writeError(w, 404, "账号不存在")
		}
		return
	}
	webAdminReply(w, nil)
}

func validWebPassword(password string) bool {
	return len(password) >= 10 && len(password) <= 128 && password != "adminadmin"
}
