package domain

import "errors"

// ErrCardNotFound: card không tồn tại hoặc không thuộc owner (deny-by-default, NFR-8).
var ErrCardNotFound = errors.New("card not found")
