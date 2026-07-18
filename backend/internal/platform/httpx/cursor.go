package httpx

import (
	"encoding/base64"
	"encoding/json"
)

// Cursor cho pagination ổn định (AD-14). Encode = base64(JSON).
type Cursor struct {
	SortKey string `json:"k"`
	ID      string `json:"i"`
}

func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return Cursor{}, err
	}
	return c, nil
}

// Page là envelope phân trang trả về cho client.
type Page struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}
