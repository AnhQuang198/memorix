package authmw

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/memorix/memorix/internal/platform/httpx"
)

const principalKey = "memorix.principal"

// RequireAuth verify Bearer access JWT; đặt Principal vào context (AD-11).
// Deny-by-default: thiếu/sai token → 401 envelope chuẩn (AD-14).
func RequireAuth(m *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || raw == "" {
			abort401(c)
			return
		}
		p, err := m.Verify(raw)
		if err != nil {
			abort401(c)
			return
		}
		c.Set(principalKey, p)
		c.Next()
	}
}

// PrincipalFrom lấy Principal đã xác thực từ context.
func PrincipalFrom(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}

// UserID là reader tiện lợi cho downstream (Sprint 2-5): trả UserID (uuid dạng
// string) của principal đã xác thực. TZ KHÔNG nằm trong principal/context —
// downstream lấy qua IdentityPort.UserTimezone(ctx, userID) (AD-9, AD-12).
func UserID(c *gin.Context) (string, bool) {
	p, ok := PrincipalFrom(c)
	return p.UserID, ok
}

// SetPrincipal đặt principal vào context — dùng cho wiring middleware và test
// double (thay cho việc set key thô "user_id"). Giữ principalKey đóng gói.
func SetPrincipal(c *gin.Context, p Principal) { c.Set(principalKey, p) }

func abort401(c *gin.Context) {
	e := httpx.NewError(httpx.CodeUnauthenticated, "authentication required").
		WithTrace(c.GetHeader("X-Request-Id"))
	c.AbortWithStatusJSON(e.HTTPStatus(), e)
}
