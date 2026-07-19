package service

import scheddom "github.com/memorix/memorix/internal/scheduling/domain"

// alias nội bộ để queue.go gọn (tránh lặp import path dài).
type cardType = scheddom.Card
type intervalType = scheddom.NextIntervals
