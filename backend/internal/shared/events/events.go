// Package events là hợp đồng domain-event dùng chung qua eventbus (Payload any).
// Publisher (review) và subscriber (progress) cùng thống nhất struct này.
package events

import "time"

// CardGradedName là tên event khớp eventbus (PascalCase quá khứ — AD conventions).
const CardGradedName = "CardGraded"

// CardGraded phát sau mỗi lần chấm thành công (ngoài TX grade — AD-8).
type CardGraded struct {
	OwnerID       string
	CardID        string
	Grade         int       // 1..4
	ScheduledDays int       // interval kế do FSRS tính
	WasNew        bool      // thẻ chưa từng ôn trước lần chấm này
	ReviewedAt    time.Time // server-ts
}
